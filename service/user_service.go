package service

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"token_blockchain/database"
)

var ErrInsufficientCredit = errors.New("insufficient credit")
var ErrUserNotFound = errors.New("user not found")

type UserService struct {
	ccService *ChaincodeService
}

func NewUserService() *UserService {
	return &UserService{
		ccService: NewChaincodeService(),
	}
}

func (s *UserService) CreateUserCredit(uc *database.UserCredit) error {
	if uc.UserID == "" {
		return errors.New("userId is required")
	}

	if err := s.ccService.SaveUserCredit(uc); err != nil {
		return fmt.Errorf("failed to save user credit to blockchain: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		if err := database.CreateUserCredit(uc); err != nil {
			return fmt.Errorf("failed to sync user credit to MongoDB: %w", err)
		}
	}

	return nil
}

func (s *UserService) GetUserCredit(userId string) (*database.UserCredit, error) {
	if s.ccService.IsMongoDBConnected() {
		uc, err := database.GetUserCredit(userId)
		if err == nil {
			return uc, nil
		}
	}

	uc, err := s.ccService.GetUserCredit(userId)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if s.ccService.IsMongoDBConnected() {
		database.UpsertUserCredit(uc)
	}

	return uc, nil
}

func (s *UserService) GetAllUserCredits() ([]*database.UserCredit, error) {
	if s.ccService.IsMongoDBConnected() {
		userCredits, err := database.GetAllUserCredits()
		if err == nil && len(userCredits) > 0 {
			return userCredits, nil
		}
	}

	userCredits, err := s.ccService.GetAllUserCredits()
	if err != nil {
		return nil, fmt.Errorf("failed to get user credits: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		for _, uc := range userCredits {
			database.UpsertUserCredit(uc)
		}
	}

	return userCredits, nil
}

func (s *UserService) UpdateUserCredit(userId string, uc *database.UserCredit) error {
	uc.UserID = userId

	if err := s.ccService.SaveUserCredit(uc); err != nil {
		return fmt.Errorf("failed to update user credit in blockchain: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		if err := database.UpdateUserCredit(userId, uc); err != nil {
			return fmt.Errorf("failed to sync user credit to MongoDB: %w", err)
		}
	}

	return nil
}

func (s *UserService) DeleteUserCredit(userId string) error {
	if err := s.ccService.DeleteUserCredit(userId); err != nil {
		return fmt.Errorf("failed to delete user credit from blockchain: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		if err := database.DeleteUserCredit(userId); err != nil {
			return fmt.Errorf("failed to delete user credit from MongoDB: %w", err)
		}
		if err := database.DeleteCreditHistoriesByUser(userId); err != nil {
			return fmt.Errorf("failed to delete credit histories from MongoDB: %w", err)
		}
	}

	return nil
}

func (s *UserService) Recharge(userId string, amount int, description string) (*database.UserCredit, error) {
	if amount <= 0 {
		return nil, errors.New("recharge amount must be positive")
	}

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

	history := &database.CreditHistory{
		ID:          uuid.New().String(),
		UserID:      userId,
		Amount:      amount,
		Type:        database.HistoryTypeRecharge,
		Description: description,
	}

	if err := s.ccService.SaveCreditHistory(history); err != nil {
		return nil, fmt.Errorf("failed to save recharge history: %w", err)
	}

	uc.Credit += amount
	uc.TotalRecharge += amount

	if err := s.ccService.SaveUserCredit(uc); err != nil {
		return nil, fmt.Errorf("failed to update user credit: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		database.CreateCreditHistory(history)
		database.UpsertUserCredit(uc)
	}

	return uc, nil
}

func (s *UserService) Consume(userId string, amount int, novelId string, description string) (*database.UserCredit, error) {
	if amount <= 0 {
		return nil, errors.New("consume amount must be positive")
	}

	uc, err := s.GetUserCredit(userId)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if uc.Credit < amount {
		return nil, ErrInsufficientCredit
	}

	history := &database.CreditHistory{
		ID:          uuid.New().String(),
		UserID:      userId,
		Amount:      amount,
		Type:        database.HistoryTypeConsume,
		Description: description,
		NovelID:     novelId,
	}

	if err := s.ccService.SaveCreditHistory(history); err != nil {
		return nil, fmt.Errorf("failed to save consume history: %w", err)
	}

	uc.Credit -= amount
	uc.TotalUsed += amount

	if err := s.ccService.SaveUserCredit(uc); err != nil {
		return nil, fmt.Errorf("failed to update user credit: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		database.CreateCreditHistory(history)
		database.UpsertUserCredit(uc)
	}

	return uc, nil
}

func (s *UserService) GetCreditHistories(userId string) ([]*database.CreditHistory, error) {
	if s.ccService.IsMongoDBConnected() {
		histories, err := database.GetCreditHistoriesByUser(userId)
		if err == nil && len(histories) > 0 {
			return histories, nil
		}
	}

	histories, err := s.ccService.GetCreditHistoriesByUser(userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get credit histories: %w", err)
	}

	return histories, nil
}
