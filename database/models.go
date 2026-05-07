package database

import "time"
//bson:"xxx" 是 MongoDB Go 驱动用的字段标签。
type Novel struct {
	ID           string `json:"id" bson:"_id,omitempty"`
	Author       string `json:"author,omitempty" bson:"author,omitempty"`
	StoryOutline string `json:"storyOutline,omitempty" bson:"storyOutline,omitempty"`
	Subsections  string `json:"subsections,omitempty" bson:"subsections,omitempty"`
	Characters   string `json:"characters,omitempty" bson:"characters,omitempty"`
	Items        string `json:"items,omitempty" bson:"items,omitempty"`
	TotalScenes  string `json:"totalScenes,omitempty" bson:"totalScenes,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

type UserCredit struct {
	ID            string `json:"id" bson:"_id,omitempty"`
	UserID        string `json:"userId" bson:"userId"`
	Credit        int    `json:"credit" bson:"credit"`
	TotalUsed     int    `json:"totalUsed" bson:"totalUsed"`
	TotalRecharge int    `json:"totalRecharge" bson:"totalRecharge"`
	CreatedAt     string `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}

type CreditHistory struct {
	ID          string `json:"id" bson:"_id,omitempty"`
	UserID      string `json:"userId" bson:"userId"`
	Amount      int    `json:"amount" bson:"amount"`
	Type        string `json:"type" bson:"type"`
	Description string `json:"description" bson:"description"`
	Timestamp   string `json:"timestamp" bson:"timestamp"`
	NovelID     string `json:"novelId,omitempty" bson:"novelId,omitempty"`
}

type User struct {
	ID                string   `json:"id" bson:"_id,omitempty"`
	Email             string   `json:"email" bson:"email"`
	Username          string   `json:"username" bson:"username"`
	PasswordHash      string   `json:"passwordHash" bson:"passwordHash"`
	DeviceFingerprint string   `json:"deviceFingerprint,omitempty" bson:"deviceFingerprint,omitempty"`
	IsActive          bool     `json:"isActive" bson:"isActive"`
	Role              string   `json:"role" bson:"role"`
	NovelIds          []string `json:"novelIds,omitempty" bson:"novelIds,omitempty"`
	CreatedAt         string   `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdatedAt         string   `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}
//全局常量作为enum存在
const (
	HistoryTypeConsume  = "consume"
	HistoryTypeRecharge = "recharge"
	HistoryTypeReward   = "reward"
)

const (
	RechargeStatusPending   = "pending"
	RechargeStatusCompleted = "completed"
	RechargeStatusFailed    = "failed"
)

type RechargeRecord struct {
	ID          string `json:"id" bson:"_id,omitempty"`
	OrderSN     string `json:"orderSn" bson:"orderSn"`
	UserID      string `json:"userId" bson:"userId"`
	Amount      int    `json:"amount" bson:"amount"`
	Status      string `json:"status" bson:"status"`
	Description string `json:"description" bson:"description"`
	CreatedAt   string `json:"createdAt" bson:"createdAt"`
	UpdatedAt   string `json:"updatedAt" bson:"updatedAt"`
}

func Now() string {
	return time.Now().Format(time.RFC3339)
}
