package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"token_blockchain/database"
	"token_blockchain/eventstore"
)

var ErrInsufficientCredit = errors.New("insufficient credit")
var ErrUserNotFound = errors.New("user not found")

type UserService struct {
	eventSvc *eventstore.EventService
}

func NewUserService() *UserService {
	return &UserService{
		eventSvc: eventstore.NewEventService(),
	}
}

func (s *UserService) CreateUserCredit(uc *database.UserCredit) error {
	if uc.UserID == "" {
		return errors.New("userId is required")
	}

	if uc.ID == "" {
		uc.ID = uuid.New().String()
	}

	ctx := context.Background()

	err := s.eventSvc.AppendEvent(ctx, eventstore.StreamUserCredit, uc.UserID, eventstore.EventUserCreditCreated, uc)
	if err != nil {
		return fmt.Errorf("failed to emit event: %w", err)
	}

	if err := database.CreateUserCredit(uc); err != nil {
		return fmt.Errorf("failed to save user credit to MongoDB: %w", err)
	}

	return nil
}

func (s *UserService) GetUserCredit(userId string) (*database.UserCredit, error) {
	ctx := context.Background()

	uc, err := database.GetUserCredit(userId)
	if err == nil {
		return uc, nil
	}

	state, err := s.eventSvc.GetLatestState(ctx, eventstore.StreamUserCredit, userId)
	if err != nil {
		return nil, ErrUserNotFound
	}

	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	var uc2 database.UserCredit
	if err := json.Unmarshal(data, &uc2); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user credit: %w", err)
	}

	return &uc2, nil
}

func (s *UserService) GetAllUserCredits() ([]*database.UserCredit, error) {
	ctx := context.Background()

	ucs, err := database.GetAllUserCredits()
	if err == nil && len(ucs) > 0 {
		return ucs, nil
	}

	states, err := s.eventSvc.GetAllLatestStates(ctx, eventstore.StreamUserCredit)
	if err != nil {
		return nil, fmt.Errorf("failed to get all user credits: %w", err)
	}

	results := make([]*database.UserCredit, 0, len(states))
	for _, state := range states {
		data, err := json.Marshal(state)
		if err != nil {
			continue
		}
		var uc database.UserCredit
		if err := json.Unmarshal(data, &uc); err == nil {
			results = append(results, &uc)
		}
	}

	return results, nil
}

func (s *UserService) UpdateUserCredit(userId string, uc *database.UserCredit) error {
	uc.UserID = userId
	ctx := context.Background()

	err := s.eventSvc.AppendEvent(ctx, eventstore.StreamUserCredit, userId, eventstore.EventUserCreditUpdated, uc)
	if err != nil {
		return fmt.Errorf("failed to emit event: %w", err)
	}

	if err := database.UpdateUserCredit(userId, uc); err != nil {
		return fmt.Errorf("failed to update user credit in MongoDB: %w", err)
	}

	return nil
}

func (s *UserService) DeleteUserCredit(userId string) error {
	ctx := context.Background()

	data := map[string]string{"userId": userId, "deleted": "true"}
	err := s.eventSvc.AppendEvent(ctx, eventstore.StreamUserCredit, userId, eventstore.EventUserCreditUpdated, data)
	if err != nil {
		return fmt.Errorf("failed to emit delete event: %w", err)
	}

	if err := database.DeleteUserCredit(userId); err != nil {
		return fmt.Errorf("failed to delete user credit from MongoDB: %w", err)
	}

	return nil
}

func (s *UserService) Recharge(userId string, amount int, description string) (*database.UserCredit, error) {
	if amount <= 0 {
		return nil, errors.New("recharge amount must be positive")
	}

	ctx := context.Background()

	uc, err := s.GetUserCredit(userId)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			uc = &database.UserCredit{
				ID:            uuid.New().String(),
				UserID:        userId,
				Credit:        0,
				TotalUsed:     0,
				TotalRecharge: 0,
			}
		} else {
			return nil, err
		}
	}

	uc.Credit += amount
	uc.TotalRecharge += amount

	eventData := map[string]interface{}{
		"userId":        userId,
		"credit":        uc.Credit,
		"amount":        amount,
		"totalRecharge": uc.TotalRecharge,
		"description":   description,
	}
	err = s.eventSvc.AppendEvent(ctx, eventstore.StreamUserCredit, userId, eventstore.EventUserCreditRecharged, eventData)
	if err != nil {
		return nil, fmt.Errorf("failed to emit recharge event: %w", err)
	}

	history := &database.CreditHistory{
		ID:          uuid.New().String(),
		UserID:      userId,
		Amount:      amount,
		Type:        database.HistoryTypeRecharge,
		Description: description,
	}
	database.CreateCreditHistory(history)
	database.UpsertUserCredit(uc)

	return uc, nil
}

func (s *UserService) Consume(userId string, amount int, novelId string, description string) (*database.UserCredit, error) {
	if userId == "" {
		return nil, errors.New("userId is required")
	}
	if amount <= 0 {
		return nil, errors.New("consume amount must be positive")
	}

	ctx := context.Background()

	uc, err := s.GetUserCredit(userId)
	if err != nil {
		log.Printf("[Consume] GetUserCredit error: %v, userId: %s", err, userId)
		return nil, ErrUserNotFound
	}

	if uc.Credit < amount {
		return nil, ErrInsufficientCredit
	}

	uc.Credit -= amount
	uc.TotalUsed += amount

	eventData := map[string]interface{}{
		"userId":     userId,
		"credit":     uc.Credit,
		"amount":     amount,
		"totalUsed":  uc.TotalUsed,
		"novelId":    novelId,
		"description": description,
	}
	err = s.eventSvc.AppendEvent(ctx, eventstore.StreamUserCredit, userId, eventstore.EventUserCreditConsumed, eventData)
	if err != nil {
		return nil, fmt.Errorf("failed to emit consume event: %w", err)
	}

	history := &database.CreditHistory{
		ID:          uuid.New().String(),
		UserID:      userId,
		Amount:      amount,
		Type:        database.HistoryTypeConsume,
		Description: description,
		NovelID:     novelId,
	}
	database.CreateCreditHistory(history)
	database.UpsertUserCredit(uc)

	return uc, nil
}

func (s *UserService) GetCreditHistories(userId string) ([]*database.CreditHistory, error) {
	histories, err := database.GetCreditHistoriesByUser(userId)
	if err == nil && len(histories) > 0 {
		return histories, nil
	}

	return nil, fmt.Errorf("credit histories not found for user: %s", userId)
}