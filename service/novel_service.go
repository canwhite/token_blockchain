package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"token_blockchain/database"
	"token_blockchain/eventstore"
)

var ErrNovelNotFound = errors.New("novel not found")

type NovelService struct {
	eventSvc *eventstore.EventService
}

func NewNovelService() *NovelService {
	return &NovelService{
		eventSvc: eventstore.NewEventService(),
	}
}

func (s *NovelService) CreateNovel(novel *database.Novel) error {
	if novel.ID == "" {
		novel.ID = uuid.New().String()
	}

	ctx := context.Background()

	err := s.eventSvc.AppendEvent(ctx, eventstore.StreamNovel, novel.ID, eventstore.EventNovelCreated, novel)
	if err != nil {
		return fmt.Errorf("failed to emit event: %w", err)
	}

	if err := database.UpsertNovel(novel); err != nil {
		return fmt.Errorf("failed to save novel to MongoDB: %w", err)
	}

	return nil
}

func (s *NovelService) GetNovel(id string) (*database.Novel, error) {
	ctx := context.Background()

	novel, err := database.GetNovel(id)
	if err == nil {
		return novel, nil
	}

	state, err := s.eventSvc.GetLatestState(ctx, eventstore.StreamNovel, id)
	if err != nil {
		return nil, ErrNovelNotFound
	}

	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	var novel2 database.Novel
	if err := json.Unmarshal(data, &novel2); err != nil {
		return nil, fmt.Errorf("failed to unmarshal novel: %w", err)
	}

	return &novel2, nil
}

func (s *NovelService) GetAllNovels() ([]*database.Novel, error) {
	ctx := context.Background()

	novels, err := database.GetAllNovels()
	if err == nil && len(novels) > 0 {
		return novels, nil
	}

	states, err := s.eventSvc.GetAllLatestStates(ctx, eventstore.StreamNovel)
	if err != nil {
		return nil, fmt.Errorf("failed to get all novels: %w", err)
	}

	results := make([]*database.Novel, 0, len(states))
	for _, state := range states {
		data, err := json.Marshal(state)
		if err != nil {
			continue
		}
		var novel database.Novel
		if err := json.Unmarshal(data, &novel); err == nil {
			results = append(results, &novel)
		}
	}

	return results, nil
}

func (s *NovelService) UpdateNovel(id string, novel *database.Novel) error {
	novel.ID = id
	ctx := context.Background()

	err := s.eventSvc.AppendEvent(ctx, eventstore.StreamNovel, id, eventstore.EventNovelUpdated, novel)
	if err != nil {
		return fmt.Errorf("failed to emit event: %w", err)
	}

	if err := database.UpdateNovel(id, novel); err != nil {
		return fmt.Errorf("failed to update novel in MongoDB: %w", err)
	}

	return nil
}

func (s *NovelService) DeleteNovel(id string) error {
	ctx := context.Background()

	data := map[string]string{"id": id, "deleted": "true"}
	err := s.eventSvc.AppendEvent(ctx, eventstore.StreamNovel, id, eventstore.EventNovelDeleted, data)
	if err != nil {
		return fmt.Errorf("failed to emit delete event: %w", err)
	}

	if err := database.DeleteNovel(id); err != nil {
		return fmt.Errorf("failed to delete novel from MongoDB: %w", err)
	}

	return nil
}