package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type DataType string

const (
	DataTypeNovel      DataType = "novel"
	DataTypeUserCredit DataType = "user_credit"
	DataTypeHistory    DataType = "credit_history"
)

type Block struct {
	Index    int      `json:"index"`
	Time     string   `json:"time"`
	DataType DataType `json:"dataType"`
	Key      string   `json:"key"`
	Value    string   `json:"value"`
	Hash     string   `json:"hash"`
	PrevHash string   `json:"prevHash"`
}

var Blockchain []Block
var mu sync.RWMutex

var indexStore = make(map[string]string)

func calculateHash(block Block) string {
	record := fmt.Sprintf("%d%s%s%s%s", block.Index, block.Time, block.DataType, block.Key, block.Value)
	h := sha256.New()
	h.Write([]byte(record))
	return hex.EncodeToString(h.Sum(nil))
}

func generateKey(dataType DataType, id string) string {
	return fmt.Sprintf("%s:%s", dataType, id)
}

func GetLatestBlock() Block {
	mu.RLock()
	defer mu.RUnlock()
	if len(Blockchain) == 0 {
		return Block{}
	}
	return Blockchain[len(Blockchain)-1]
}

func Write(dataType DataType, key string, value string) (Block, error) {
	mu.Lock()
	defer mu.Unlock()

	t := time.Now()
	newBlock := Block{
		Index:    len(Blockchain),
		Time:     t.String(),
		DataType: dataType,
		Key:      key,
		Value:    value,
		PrevHash: GetLatestBlock().Hash,
	}
	newBlock.Hash = calculateHash(newBlock)

	Blockchain = append(Blockchain, newBlock)
	indexStore[key] = newBlock.Hash

	return newBlock, nil
}

func Read(dataType DataType, key string) (string, error) {
	mu.RLock()
	defer mu.RUnlock()

	fullKey := generateKey(dataType, key)
	for _, block := range Blockchain {
		if block.DataType == dataType && block.Key == fullKey {
			return block.Value, nil
		}
	}
	return "", fmt.Errorf("not found: %s", fullKey)
}

func ReadAll(dataType DataType) ([]string, error) {
	mu.RLock()
	defer mu.RUnlock()

	var results []string
	for _, block := range Blockchain {
		if block.DataType == dataType {
			results = append(results, block.Value)
		}
	}
	return results, nil
}

func Delete(dataType DataType, key string) error {
	mu.Lock()
	defer mu.Unlock()

	fullKey := generateKey(dataType, key)
	for i, block := range Blockchain {
		if block.DataType == dataType && block.Key == fullKey {
			Blockchain = append(Blockchain[:i], Blockchain[i+1:]...)
			reindexBlockchain()
			return nil
		}
	}
	return fmt.Errorf("not found: %s", fullKey)
}

func reindexBlockchain() {
	indexStore = make(map[string]string)
	for i := range Blockchain {
		Blockchain[i].Index = i
		if i > 0 {
			Blockchain[i].PrevHash = Blockchain[i-1].Hash
		}
		Blockchain[i].Hash = calculateHash(Blockchain[i])
		indexStore[Blockchain[i].Key] = Blockchain[i].Hash
	}
}

func KeyExists(dataType DataType, key string) bool {
	mu.RLock()
	defer mu.RUnlock()

	fullKey := generateKey(dataType, key)
	_, exists := indexStore[fullKey]
	return exists
}

func IsBlockValid(newBlock Block, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}
	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}
	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

func ReplaceChain(newBlocks []Block) {
	mu.Lock()
	defer mu.Unlock()

	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
		reindexBlockchain()
	}
}

func GetAllKeys(dataType DataType) []string {
	mu.RLock()
	defer mu.RUnlock()

	var keys []string
	for _, block := range Blockchain {
		if block.DataType == dataType {
			keys = append(keys, block.Key)
		}
	}
	return keys
}

func MarshalValue(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func UnmarshalValue(value string, v interface{}) error {
	return json.Unmarshal([]byte(value), v)
}

func init() {
	if len(Blockchain) == 0 {
		t := time.Now()
		genesisBlock := Block{
			Index:    0,
			Time:     t.String(),
			DataType: "genesis",
			Key:      "genesis:block",
			Value:    "{}",
			Hash:     "",
			PrevHash: "",
		}
		genesisBlock.Hash = calculateHash(genesisBlock)
		Blockchain = append(Blockchain, genesisBlock)
		indexStore[genesisBlock.Key] = genesisBlock.Hash
	}
}
