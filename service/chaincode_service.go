package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"token_blockchain/blockchain"
	"token_blockchain/database"
)

type ChaincodeService struct{}

func NewChaincodeService() *ChaincodeService {
	return &ChaincodeService{}
}

func (s *ChaincodeService) SaveNovel(novel *database.Novel) error {
	value, err := json.Marshal(novel)
	if err != nil {
		return fmt.Errorf("failed to marshal novel: %w", err)
	}

	key := fmt.Sprintf("%s:%s", blockchain.DataTypeNovel, novel.ID)
	_, err = blockchain.Write(blockchain.DataTypeNovel, key, string(value))
	return err
}

func (s *ChaincodeService) GetNovel(id string) (*database.Novel, error) {
	key := fmt.Sprintf("%s:%s", blockchain.DataTypeNovel, id)
	value, err := blockchain.Read(blockchain.DataTypeNovel, key)
	if err != nil {
		return nil, err
	}

	var novel database.Novel
	if err := json.Unmarshal([]byte(value), &novel); err != nil {
		return nil, fmt.Errorf("failed to unmarshal novel: %w", err)
	}
	return &novel, nil
}

func (s *ChaincodeService) GetAllNovels() ([]*database.Novel, error) {
	values, err := blockchain.ReadAll(blockchain.DataTypeNovel)
	if err != nil {
		return nil, err
	}

	novels := make([]*database.Novel, 0, len(values))
	for _, v := range values {
		var novel database.Novel
		if err := json.Unmarshal([]byte(v), &novel); err != nil {
			continue
		}
		novels = append(novels, &novel)
	}
	return novels, nil
}

func (s *ChaincodeService) DeleteNovel(id string) error {
	key := fmt.Sprintf("%s:%s", blockchain.DataTypeNovel, id)
	return blockchain.Delete(blockchain.DataTypeNovel, key)
}

func (s *ChaincodeService) SaveUserCredit(uc *database.UserCredit) error {
	value, err := json.Marshal(uc)
	if err != nil {
		return fmt.Errorf("failed to marshal user credit: %w", err)
	}

	key := fmt.Sprintf("%s:%s", blockchain.DataTypeUserCredit, uc.UserID)
	_, err = blockchain.Write(blockchain.DataTypeUserCredit, key, string(value))
	return err
}

func (s *ChaincodeService) GetUserCredit(userId string) (*database.UserCredit, error) {
	key := fmt.Sprintf("%s:%s", blockchain.DataTypeUserCredit, userId)
	value, err := blockchain.Read(blockchain.DataTypeUserCredit, key)
	if err != nil {
		return nil, err
	}

	var uc database.UserCredit
	if err := json.Unmarshal([]byte(value), &uc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user credit: %w", err)
	}
	return &uc, nil
}

func (s *ChaincodeService) GetAllUserCredits() ([]*database.UserCredit, error) {
	values, err := blockchain.ReadAll(blockchain.DataTypeUserCredit)
	if err != nil {
		return nil, err
	}

	userCredits := make([]*database.UserCredit, 0, len(values))
	for _, v := range values {
		var uc database.UserCredit
		if err := json.Unmarshal([]byte(v), &uc); err != nil {
			continue
		}
		userCredits = append(userCredits, &uc)
	}
	return userCredits, nil
}

func (s *ChaincodeService) DeleteUserCredit(userId string) error {
	key := fmt.Sprintf("%s:%s", blockchain.DataTypeUserCredit, userId)
	return blockchain.Delete(blockchain.DataTypeUserCredit, key)
}

func (s *ChaincodeService) SaveCreditHistory(h *database.CreditHistory) error {
	value, err := json.Marshal(h)
	if err != nil {
		return fmt.Errorf("failed to marshal credit history: %w", err)
	}

	key := fmt.Sprintf("%s:%s", blockchain.DataTypeHistory, h.ID)
	_, err = blockchain.Write(blockchain.DataTypeHistory, key, string(value))
	return err
}

func (s *ChaincodeService) GetCreditHistoriesByUser(userId string) ([]*database.CreditHistory, error) {
	values, err := blockchain.ReadAll(blockchain.DataTypeHistory)
	if err != nil {
		return nil, err
	}

	histories := make([]*database.CreditHistory, 0)
	for _, v := range values {
		var h database.CreditHistory
		if err := json.Unmarshal([]byte(v), &h); err != nil {
			continue
		}
		if h.UserID == userId {
			histories = append(histories, &h)
		}
	}
	return histories, nil
}

func (s *ChaincodeService) ExistsNovel(id string) bool {
	return blockchain.KeyExists(blockchain.DataTypeNovel, fmt.Sprintf("%s:%s", blockchain.DataTypeNovel, id))
}

func (s *ChaincodeService) ExistsUserCredit(userId string) bool {
	return blockchain.KeyExists(blockchain.DataTypeUserCredit, fmt.Sprintf("%s:%s", blockchain.DataTypeUserCredit, userId))
}

func (s *ChaincodeService) IsMongoDBConnected() bool {
	return database.GetMongoDB() != nil
}

func ParseNovelID(key string) string {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return key
}

func ParseUserID(key string) string {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return key
}
