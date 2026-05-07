package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)
//全局变量
var (
	client   *mongo.Client
	database *mongo.Database
	mu       sync.RWMutex
)
//collectionNames，全局常量
const (
	CollectionNovels          = "novels"
	CollectionUserCredits     = "user_credits"
	CollectionCreditHistories = "credit_histories"
	CollectionUsers           = "users"
	CollectionRechargeRecords = "recharge_records"
)
//初始化mongodb
func InitMongoDB(uri, dbName string) error {
	mu.Lock()
	defer mu.Unlock()
	//解决内存未释放的问题
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	clientOptions := options.Client().ApplyURI(uri)
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database = client.Database(dbName)
	return nil
}

func GetMongoDB() *mongo.Database {
	mu.RLock()
	defer mu.RUnlock()
	return database
}

func CloseMongoDB() error {
	mu.Lock()
	defer mu.Unlock()

	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return client.Disconnect(ctx)
	}
	return nil
}

func GetNovelsCollection() *mongo.Collection {
	return GetMongoDB().Collection(CollectionNovels)
}

func GetUserCreditsCollection() *mongo.Collection {
	return GetMongoDB().Collection(CollectionUserCredits)
}

func GetCreditHistoriesCollection() *mongo.Collection {
	return GetMongoDB().Collection(CollectionCreditHistories)
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
		bson.M{"id": novel.ID},
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
	return GetMongoDB().Collection(CollectionUsers)
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
	return GetMongoDB().Collection(CollectionRechargeRecords)
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
