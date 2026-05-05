package service

import (
	"fmt"

	"github.com/google/uuid"
	"token_blockchain/database"
)

type NovelService struct {
	ccService *ChaincodeService
}

func NewNovelService() *NovelService {
	return &NovelService{
		ccService: NewChaincodeService(),
	}
}

func (s *NovelService) CreateNovel(novel *database.Novel) error {
	if novel.ID == "" {
		novel.ID = uuid.New().String()
	}

	if err := s.ccService.SaveNovel(novel); err != nil {
		return fmt.Errorf("failed to save novel to blockchain: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		if err := database.CreateNovel(novel); err != nil {
			return fmt.Errorf("failed to sync novel to MongoDB: %w", err)
		}
	}

	return nil
}

func (s *NovelService) GetNovel(id string) (*database.Novel, error) {
	if s.ccService.IsMongoDBConnected() {
		novel, err := database.GetNovel(id)
		if err == nil {
			return novel, nil
		}
	}

	novel, err := s.ccService.GetNovel(id)
	if err != nil {
		return nil, fmt.Errorf("novel not found: %s", id)
	}

	if s.ccService.IsMongoDBConnected() {
		database.UpsertNovel(novel)
	}

	return novel, nil
}

func (s *NovelService) GetAllNovels() ([]*database.Novel, error) {
	if s.ccService.IsMongoDBConnected() {
		novels, err := database.GetAllNovels()
		if err == nil && len(novels) > 0 {
			return novels, nil
		}
	}

	novels, err := s.ccService.GetAllNovels()
	if err != nil {
		return nil, fmt.Errorf("failed to get novels: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		for _, novel := range novels {
			database.UpsertNovel(novel)
		}
	}

	return novels, nil
}

func (s *NovelService) UpdateNovel(id string, novel *database.Novel) error {
	novel.ID = id

	if err := s.ccService.SaveNovel(novel); err != nil {
		return fmt.Errorf("failed to update novel in blockchain: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		if err := database.UpdateNovel(id, novel); err != nil {
			return fmt.Errorf("failed to sync novel to MongoDB: %w", err)
		}
	}

	return nil
}

func (s *NovelService) DeleteNovel(id string) error {
	if err := s.ccService.DeleteNovel(id); err != nil {
		return fmt.Errorf("failed to delete novel from blockchain: %w", err)
	}

	if s.ccService.IsMongoDBConnected() {
		if err := database.DeleteNovel(id); err != nil {
			return fmt.Errorf("failed to delete novel from MongoDB: %w", err)
		}
	}

	return nil
}
