package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"token_blockchain/database"
)

// Event types
const (
	EventUserCreditCreated   = "UserCreditCreated"
	EventUserCreditUpdated   = "UserCreditUpdated"
	EventUserCreditRecharged = "UserCreditRecharged"
	EventUserCreditConsumed = "UserCreditConsumed"
	EventNovelCreated       = "NovelCreated"
	EventNovelUpdated       = "NovelUpdated"
	EventNovelDeleted       = "NovelDeleted"
)

// Stream names
const (
	StreamUserCredit = "userCredit"
	StreamNovel     = "novel"
)

// EventData represents an event in memory
type EventData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	StreamName string          `json:"streamName"`
	StreamID   string          `json:"streamId"`
	Version    int64           `json:"version"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// EventService handles event operations with MongoDB persistence
type EventService struct {
	mu   sync.RWMutex
	subs map[string][]chan EventData // streamName -> channels
}

var (
	instance *EventService
	once     sync.Once
)

// NewEventService creates singleton EventService
func NewEventService() *EventService {
	once.Do(func() {
		instance = &EventService{
			subs: make(map[string][]chan EventData),
		}
	})
	return instance
}

// GetEventService returns the singleton instance
func GetEventService() *EventService {
	return instance
}

// AppendEvent appends an event to a stream and persists to MongoDB
func (s *EventService) AppendEvent(ctx context.Context, streamName string, streamID string, eventType string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	// Get current version
	version := int64(s.getEventCount(streamName, streamID))

	eventDoc := &database.EventDocument{
		ID:        uuid.New().String(),
		Type:      eventType,
		StreamName: streamName,
		StreamID:   streamID,
		Data:       jsonData,
		Version:    version,
		CreatedAt:  time.Now(),
	}

	// Persist to MongoDB
	if err := database.CreateEvent(ctx, eventDoc); err != nil {
		return fmt.Errorf("failed to save event to MongoDB: %w", err)
	}

	// Create EventData for subscribers
	eventData := EventData{
		ID:         eventDoc.ID,
		Type:       eventType,
		Data:       jsonData,
		StreamName: streamName,
		StreamID:   streamID,
		Version:    version,
		CreatedAt:  eventDoc.CreatedAt,
	}

	// Notify subscribers asynchronously
	// emit and then notify subscription
	go s.notifySubscribers(streamName, eventData)

	return nil
}

// getEventCount returns the number of events in a stream from MongoDB
func (s *EventService) getEventCount(streamName, streamID string) int {
	count, err := database.CountEvents(streamName, streamID)
	if err != nil {
		return 0
	}
	return count
}

// ReadEvents reads events from MongoDB (newest first)
func (s *EventService) ReadEvents(ctx context.Context, streamName string, streamID string) ([]EventData, error) {
	events, err := database.GetEventsByStream(streamName, streamID)
	if err != nil {
		return nil, err
	}

	// Reverse to get newest first
	result := make([]EventData, len(events))
	for i, e := range events {
		result[len(events)-1-i] = EventData{
			ID:         e.ID,
			Type:       e.Type,
			Data:       e.Data,
			StreamName: e.StreamName,
			StreamID:   e.StreamID,
			Version:    e.Version,
			CreatedAt:  e.CreatedAt,
		}
	}

	return result, nil
}

// GetLatestState returns the latest state for a stream (from latest event)
func (s *EventService) GetLatestState(ctx context.Context, streamName string, streamID string) (interface{}, error) {
	events, err := database.GetLatestEvent(streamName, streamID)
	if err != nil {
		return nil, err
	}

	if events == nil {
		return nil, fmt.Errorf("no events found for stream %s-%s", streamName, streamID)
	}

	var data interface{}
	if err := json.Unmarshal(events.Data, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// GetAllLatestStates returns all latest states for a stream type
func (s *EventService) GetAllLatestStates(ctx context.Context, streamName string) ([]interface{}, error) {
	events, err := database.GetLatestEventsByStream(streamName)
	if err != nil {
		return nil, err
	}

	results := make([]interface{}, 0, len(events))
	for _, e := range events {
		var data interface{}
		if err := json.Unmarshal(e.Data, &data); err == nil {
			results = append(results, data)
		}
	}

	return results, nil
}

// Subscribe creates a subscription to a stream
func (s *EventService) Subscribe(streamName string) (<-chan EventData, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan EventData, 100)
	s.subs[streamName] = append(s.subs[streamName], ch)

	cleanup := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		chans := s.subs[streamName]
		for i, c := range chans {
			if c == ch {
				s.subs[streamName] = append(chans[:i], chans[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, cleanup
}

// notifySubscribers notifies all subscribers of a stream
func (s *EventService) notifySubscribers(streamName string, event EventData) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for name, chans := range s.subs {
		if name == streamName || name == "*" {
			for _, ch := range chans {
				select {
				case ch <- event:
				default:
				}
			}
		}
	}
}

// StreamExists checks if a stream has any events
func (s *EventService) StreamExists(ctx context.Context, streamName string, streamID string) bool {
	return s.getEventCount(streamName, streamID) > 0
}

// GetStreamCount returns the number of events in a stream
func (s *EventService) GetStreamCount(ctx context.Context, streamName string, streamID string) (int, error) {
	return s.getEventCount(streamName, streamID), nil
}

// Close cleans up all subscriptions
func (s *EventService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, chans := range s.subs {
		for _, ch := range chans {
			close(ch)
		}
	}
	s.subs = make(map[string][]chan EventData)
}
