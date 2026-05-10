package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBConfig struct {
	URI            string
	Database       string
	Timeout        time.Duration
	MaxPoolSize    uint64
	MinPoolSize    uint64
	MaxConnIdleTTL time.Duration
}

func DefaultMongoDBConfig() *MongoDBConfig {
	return &MongoDBConfig{
		URI:            "mongodb://localhost:27017",
		Database:       "token_blockchain",
		Timeout:        10 * time.Second,
		MaxPoolSize:    10,
		MinPoolSize:    2,
		MaxConnIdleTTL: 30 * time.Minute,
	}
}

type MongoDBInstance struct {
	client   *mongo.Client
	database *mongo.Database
	config   *MongoDBConfig
	mu       sync.RWMutex
}

var (
	mongoInstance *MongoDBInstance
	mongoOnce     sync.Once
)
//collectionNames，全局常量
const (
	CollectionNovels          = "novels"
	CollectionUserCredits     = "user_credits"
	CollectionCreditHistories = "credit_histories"
	CollectionUsers           = "users"
	CollectionRechargeRecords = "recharge_records"
	CollectionEvents          = "events"
)
func loadMongoConfig() *MongoDBConfig {
	if err := godotenv.Load(); err != nil {
		log.Printf("未找到 .env 文件，使用系统环境变量: %v", err)
	}

	config := DefaultMongoDBConfig()

	if uri := os.Getenv("MONGODB_URI"); uri != "" {
		config.URI = uri
	}

	if database := os.Getenv("MONGODB_DATABASE"); database != "" {
		config.Database = database
	}

	if timeout := os.Getenv("MONGODB_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout); err == nil {
			config.Timeout = duration
		}
	}

	if maxPool := os.Getenv("MONGODB_MAX_POOL_SIZE"); maxPool != "" {
		if size, err := strconv.ParseUint(maxPool, 10, 64); err == nil {
			config.MaxPoolSize = size
		}
	}

	if minPool := os.Getenv("MONGODB_MIN_POOL_SIZE"); minPool != "" {
		if size, err := strconv.ParseUint(minPool, 10, 64); err == nil {
			config.MinPoolSize = size
		}
	}

	if idleTTL := os.Getenv("MONGODB_MAX_CONN_IDLE_TTL"); idleTTL != "" {
		if duration, err := time.ParseDuration(idleTTL); err == nil {
			config.MaxConnIdleTTL = duration
		}
	}

	return config
}

func GetMongoInstance() *MongoDBInstance {
	mongoOnce.Do(func() {
		config := loadMongoConfig()

		mongoInstance = &MongoDBInstance{
			config: config,
		}

		if err := mongoInstance.Connect(); err != nil {
			log.Fatalf("MongoDB自动连接失败: %v", err)
		}

		log.Printf("MongoDB自动连接成功! 数据库: %s", config.Database)
	})
	return mongoInstance
}

func (m *MongoDBInstance) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client != nil {
		return nil
	}

	clientOptions := options.Client().ApplyURI(m.config.URI)
	clientOptions.SetMaxPoolSize(m.config.MaxPoolSize)
	clientOptions.SetMinPoolSize(m.config.MinPoolSize)
	clientOptions.SetMaxConnIdleTime(m.config.MaxConnIdleTTL)
	clientOptions.SetConnectTimeout(m.config.Timeout)
	clientOptions.SetServerSelectionTimeout(m.config.Timeout)

	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("连接MongoDB失败: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("MongoDB连接测试失败: %v", err)
	}

	m.client = client
	m.database = client.Database(m.config.Database)

	log.Printf("MongoDB连接成功! 数据库: %s", m.config.Database)
	return nil
}

func (m *MongoDBInstance) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	err := m.client.Disconnect(ctx)
	if err != nil {
		return fmt.Errorf("断开MongoDB连接失败: %v", err)
	}

	m.client = nil
	m.database = nil
	log.Println("MongoDB连接已断开")
	return nil
}

func (m *MongoDBInstance) GetClient() *mongo.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.client == nil {
		log.Fatal("MongoDB未初始化，请先调用Connect()")
	}
	return m.client
}

func (m *MongoDBInstance) GetDatabase() *mongo.Database {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.database == nil {
		log.Fatal("MongoDB数据库未初始化，请先调用Connect()")
	}
	return m.database
}

func (m *MongoDBInstance) GetCollection(name string) *mongo.Collection {
	return m.GetDatabase().Collection(name)
}

func (m *MongoDBInstance) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := m.client.Ping(ctx, nil)
	return err == nil
}

func (m *MongoDBInstance) Close() error {
	return m.Disconnect()
}

func GetMongoDB() *mongo.Database {
	return GetMongoInstance().GetDatabase()
}

func CloseMongoDB() error {
	return GetMongoInstance().Close()
}

func GetNovelsCollection() *mongo.Collection {
	return GetMongoInstance().GetCollection(CollectionNovels)
}

func GetUserCreditsCollection() *mongo.Collection {
	return GetMongoInstance().GetCollection(CollectionUserCredits)
}

func GetCreditHistoriesCollection() *mongo.Collection {
	return GetMongoInstance().GetCollection(CollectionCreditHistories)
}

func CreateNovel(novel *Novel) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	novel.CreatedAt = Now()
	novel.UpdatedAt = Now()
	_, err := GetNovelsCollection().InsertOne(ctx, novel)
	return err
}

func GetNovel(id string) (*Novel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var novel Novel
	err := GetNovelsCollection().FindOne(ctx, bson.M{"id": id}).Decode(&novel)
	if err != nil {
		return nil, err
	}
	return &novel, nil
}

func GetAllNovels() ([]*Novel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := GetNovelsCollection().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var novels []*Novel
	if err = cursor.All(ctx, &novels); err != nil {
		return nil, err
	}
	return novels, nil
}

func UpdateNovel(id string, novel *Novel) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	novel.UpdatedAt = Now()
	_, err := GetNovelsCollection().UpdateOne(
		ctx,
		bson.M{"id": id},
		bson.M{"$set": novel},
	)
	return err
}

func DeleteNovel(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := GetNovelsCollection().DeleteOne(ctx, bson.M{"id": id})
	return err
}

func UpsertNovel(novel *Novel) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	novel.UpdatedAt = Now()
	opts := options.Update().SetUpsert(true)
	_, err := GetNovelsCollection().UpdateOne(
		ctx,
		bson.M{"_id": novel.ID},
		bson.M{"$set": novel},
		opts,
	)
	return err
}

func CreateUserCredit(uc *UserCredit) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uc.CreatedAt = Now()
	uc.UpdatedAt = Now()
	_, err := GetUserCreditsCollection().InsertOne(ctx, uc)
	return err
}

func GetUserCredit(userId string) (*UserCredit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var uc UserCredit
	err := GetUserCreditsCollection().FindOne(ctx, bson.M{"userId": userId}).Decode(&uc)
	if err != nil {
		return nil, err
	}
	return &uc, nil
}

func GetAllUserCredits() ([]*UserCredit, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := GetUserCreditsCollection().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var userCredits []*UserCredit
	if err = cursor.All(ctx, &userCredits); err != nil {
		return nil, err
	}
	return userCredits, nil
}

func UpdateUserCredit(userId string, uc *UserCredit) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uc.UpdatedAt = Now()
	_, err := GetUserCreditsCollection().UpdateOne(
		ctx,
		bson.M{"userId": userId},
		bson.M{"$set": uc},
	)
	return err
}

func DeleteUserCredit(userId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := GetUserCreditsCollection().DeleteOne(ctx, bson.M{"userId": userId})
	return err
}

func UpsertUserCredit(uc *UserCredit) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uc.UpdatedAt = Now()
	opts := options.Update().SetUpsert(true)
	_, err := GetUserCreditsCollection().UpdateOne(
		ctx,
		bson.M{"userId": uc.UserID},
		bson.M{"$set": uc},
		opts,
	)
	return err
}

func CreateCreditHistory(h *CreditHistory) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.Timestamp = Now()
	_, err := GetCreditHistoriesCollection().InsertOne(ctx, h)
	return err
}

func GetCreditHistoriesByUser(userId string) ([]*CreditHistory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := GetCreditHistoriesCollection().Find(ctx, bson.M{"userId": userId})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var histories []*CreditHistory
	if err = cursor.All(ctx, &histories); err != nil {
		return nil, err
	}
	return histories, nil
}

func DeleteCreditHistoriesByUser(userId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := GetCreditHistoriesCollection().DeleteMany(ctx, bson.M{"userId": userId})
	return err
}

func GetUsersCollection() *mongo.Collection {
	return GetMongoInstance().GetCollection(CollectionUsers)
}

func CreateUser(user *User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user.CreatedAt = Now()
	user.UpdatedAt = Now()
	_, err := GetUsersCollection().InsertOne(ctx, user)
	return err
}

func GetUser(id string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	err := GetUsersCollection().FindOne(ctx, bson.M{"id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByEmail(email string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user User
	err := GetUsersCollection().FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetAllUsers() ([]*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := GetUsersCollection().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []*User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func UpdateUser(id string, user *User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user.UpdatedAt = Now()
	_, err := GetUsersCollection().UpdateOne(
		ctx,
		bson.M{"id": id},
		bson.M{"$set": user},
	)
	return err
}

func DeleteUser(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := GetUsersCollection().DeleteOne(ctx, bson.M{"id": id})
	return err
}

func UpsertUser(user *User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user.UpdatedAt = Now()
	opts := options.Update().SetUpsert(true)
	_, err := GetUsersCollection().UpdateOne(
		ctx,
		bson.M{"id": user.ID},
		bson.M{"$set": user},
		opts,
	)
	return err
}

func GetRechargeRecordsCollection() *mongo.Collection {
	return GetMongoInstance().GetCollection(CollectionRechargeRecords)
}

func CreateRechargeRecord(record *RechargeRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	record.CreatedAt = Now()
	record.UpdatedAt = Now()
	_, err := GetRechargeRecordsCollection().InsertOne(ctx, record)
	return err
}

func GetRechargeRecordByOrderSN(orderSN string) (*RechargeRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var record RechargeRecord
	err := GetRechargeRecordsCollection().FindOne(ctx, bson.M{"orderSn": orderSN}).Decode(&record)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func UpdateRechargeRecordStatus(orderSN string, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := GetRechargeRecordsCollection().UpdateOne(
		ctx,
		bson.M{"orderSn": orderSN},
		bson.M{"$set": bson.M{"status": status, "updatedAt": Now()}},
	)
	return err
}

func UpsertRechargeRecord(record *RechargeRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	record.UpdatedAt = Now()
	opts := options.Update().SetUpsert(true)
	_, err := GetRechargeRecordsCollection().UpdateOne(
		ctx,
		bson.M{"orderSn": record.OrderSN},
		bson.M{"$set": record},
		opts,
	)
	return err
}

func GetEventsCollection() *mongo.Collection {
	return GetMongoInstance().GetCollection(CollectionEvents)
}

// CreateEvent saves an event to MongoDB
func CreateEvent(ctx context.Context, doc *EventDocument) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := GetEventsCollection().InsertOne(ctx, doc)
	return err
}

// GetEventsByStream returns all events for a stream
func GetEventsByStream(streamName, streamID string) ([]*EventDocument, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"streamName": streamName,
		"streamId":   streamID,
	}

	cursor, err := GetEventsCollection().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "version", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []*EventDocument
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

// GetLatestEvent returns the latest event for a stream
func GetLatestEvent(streamName, streamID string) (*EventDocument, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"streamName": streamName,
		"streamId":   streamID,
	}

	var event EventDocument
	err := GetEventsCollection().FindOne(ctx, filter, options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})).Decode(&event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetLatestEventsByStream returns the latest event for each stream ID in a stream
func GetLatestEventsByStream(streamName string) ([]*EventDocument, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Aggregation pipeline to get latest event per stream ID
	pipeline := []bson.M{
		{"$match": bson.M{"streamName": streamName}},
		{"$sort": bson.D{{Key: "version", Value: -1}}},
		{"$group": bson.M{
			"_id":        "$streamId",
			"id":         bson.M{"$first": "$_id"},
			"type":       bson.M{"$first": "$type"},
			"streamName": bson.M{"$first": "$streamName"},
			"streamId":   bson.M{"$first": "$streamId"},
			"data":       bson.M{"$first": "$data"},
			"version":   bson.M{"$first": "$version"},
			"createdAt": bson.M{"$first": "$createdAt"},
		}},
	}

	cursor, err := GetEventsCollection().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	events := make([]*EventDocument, 0, len(results))
	for _, r := range results {
		events = append(events, &EventDocument{
			ID:         getStringFromBson(r, "id"),
			Type:       getStringFromBson(r, "type"),
			StreamName: getStringFromBson(r, "streamName"),
			StreamID:   getStringFromBson(r, "streamId"),
			Data:       getBytesFromBson(r, "data"),
			Version:    getInt64FromBson(r, "version"),
			CreatedAt:  getTimeFromBson(r, "createdAt"),
		})
	}

	return events, nil
}

// CountEvents returns the count of events for a stream
func CountEvents(streamName, streamID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"streamName": streamName,
		"streamId":   streamID,
	}

	count, err := GetEventsCollection().CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// Helper functions for bson
func getStringFromBson(m bson.M, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getBytesFromBson(m bson.M, key string) []byte {
	if v, ok := m[key]; ok {
		if b, ok := v.(primitive.Binary); ok {
			return b.Data
		}
	}
	return nil
}

func getInt64FromBson(m bson.M, key string) int64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int64:
			return val
		case int:
			return int64(val)
		}
	}
	return 0
}

func getTimeFromBson(m bson.M, key string) time.Time {
	if v, ok := m[key]; ok {
		if t, ok := v.(time.Time); ok {
			return t
		}
	}
	return time.Time{}
}
