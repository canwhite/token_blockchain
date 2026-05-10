# Event 模块解决方案

## 1. 问题回顾

### 原始问题
| 问题 | 原因 | 影响 |
|------|------|------|
| 数据丢失 | blockchain 是内存存储 | 服务重启后用户数据消失 |
| 读取性能差 | Read 扫描整个链 O(n) | 数据量大时很慢 |
| 无事件监听 | 没有 Event 机制 | 前端无法实时接收变更 |

### 尝试过的方案

| 方案 | 结果 | 原因 |
|------|------|------|
| JSON 文件持久化 | 部分解决 | 但读取仍是 O(n)，无事件机制 |
| EventStoreDB | 放弃 | 需要额外进程，架构变复杂 |
| 内存版 EventStore | 放弃 | 服务重启数据丢失 |

---

## 2. 最终方案：MongoDB + Event 模块

### 架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                            MongoDB                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │  events 集合 (Event Document)                               │     │
│  │  ├── _id: string (UUID)                                    │     │
│  │  ├── type: string (EventType)                              │     │
│  │  ├── streamName: string (e.g. "userCredit")                │     │
│  │  ├── streamId: string (e.g. "user123")                     │     │
│  │  ├── data: binary (JSON)                                   │     │
│  │  ├── version: int64                                        │     │
│  │  └── createdAt: timestamp                                  │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │  业务数据集合                                               │     │
│  │  ├── user_credits (用户积分)                               │     │
│  │  ├── novels (小说)                                         │     │
│  │  ├── credit_histories (积分历史)                           │     │
│  │  ├── recharge_records (充值记录)                           │     │
│  │  └── users (用户)                                          │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
                              ↑
                              │
                    AppendEvent() → 持久化事件
                              │
                              ↓
                    ┌─────────────────────┐
                    │    EventService     │
                    │  (内存订阅管理)     │
                    └─────────────────────┘
                              │
                    Subscribe() → 内存 channel
                              │
                              ↓
                    ┌─────────────────────┐
                    │   SSE Endpoint      │
                    │  /api/v1/events    │
                    └─────────────────────┘
```

---

## 3. 核心组件

### 3.1 EventService (`eventstore/client.go`)

```go
type EventService struct {
    mu   sync.RWMutex
    subs map[string][]chan EventData  // streamName -> channels
}

// 主要方法
func (s *EventService) AppendEvent(ctx, streamName, streamID, eventType, data) error
func (s *EventService) ReadEvents(ctx, streamName, streamID) ([]EventData, error)
func (s *EventService) GetLatestState(ctx, streamName, streamID) (interface{}, error)
func (s *EventService) GetAllLatestStates(ctx, streamName) ([]interface{}, error)
func (s *EventService) Subscribe(streamName) (<-chan EventData, func())
```

### 3.2 Event 类型常量 (`eventstore/client.go`)

```go
const (
    EventUserCreditCreated   = "UserCreditCreated"
    EventUserCreditUpdated   = "UserCreditUpdated"
    EventUserCreditRecharged = "UserCreditRecharged"
    EventUserCreditConsumed  = "UserCreditConsumed"
    EventNovelCreated        = "NovelCreated"
    EventNovelUpdated        = "NovelUpdated"
    EventNovelDeleted        = "NovelDeleted"
)
```

### 3.3 MongoDB Event 操作 (`database/mongodb.go`)

```go
// 保存事件到 MongoDB
func CreateEvent(ctx context.Context, doc *EventDocument) error

// 按流读取所有事件（按版本升序）
func GetEventsByStream(streamName, streamID string) ([]*EventDocument, error)

// 获取流中最新事件
func GetLatestEvent(streamName, streamID string) (*EventDocument, error)

// 获取所有流的最新事件（用于重建聚合）
func GetLatestEventsByStream(streamName string) ([]*EventDocument, error)

// 统计事件数量
func CountEvents(streamName, streamID string) (int, error)
```

### 3.4 EventDocument 模型 (`database/models.go`)

```go
type EventDocument struct {
    ID         string    `json:"id" bson:"_id"`
    Type       string    `json:"type" bson:"type"`
    StreamName string    `json:"streamName" bson:"streamName"`
    StreamID   string    `json:"streamId" bson:"streamId"`
    Data       []byte    `json:"data" bson:"data"`
    Version    int64     `json:"version" bson:"version"`
    CreatedAt  time.Time `json:"createdAt" bson:"createdAt"`
}
```

---

## 4. 数据流

### 4.1 写入流程

```
客户端
  │
  ▼
API Handler
  │
  ▼
Service Layer
  │
  ├─► eventstore.AppendEvent()
  │         │
  │         ├─► 序列化 data 为 JSON
  │         │
  │         ├─► 获取当前 version (事件数量)
  │         │
  │         ├─► 创建 EventDocument
  │         │
  │         └─► database.CreateEvent() → MongoDB
  │
  └─► 业务逻辑 (更新 MongoDB 业务集合)
  │
  ▼
异步: notifySubscribers() → 内存 channel
```

### 4.2 读取流程

```
客户端 GET /api/v1/users/:id
  │
  ▼
API Handler
  │
  ▼
Service Layer → GetUserCredit()
  │
  ├─► database.GetUserCredit() (优先 MongoDB)
  │
  └─► 如果 MongoDB 没有 → eventstore.GetLatestState()
                                              │
                                              ▼
                                    从 MongoDB events 获取最新事件
                                    解析 data 字段为业务对象
```

### 4.3 事件订阅流程 (SSE)

```
客户端 GET /api/v1/events/listen
  │
  ▼
eventstore.Subscribe("userCredit")
  │
  ▼
返回 <-chan EventData + cleanup 函数
  │
  ▼
SSE Handler 循环:
  for event := range ch {
      推送 event 到客户端
  }
```

---

## 5. Stream 设计

### 5.1 Stream 命名规则

```
streamName-streamID

例:
userCredit-69ff438c6e3717d077de9996
novel-a1b2c3d4-e5f6
creditHistory-12345678
```

### 5.2 业务 Stream 映射

| Stream Name | Stream ID 示例 | 用途 |
|-------------|---------------|------|
| userCredit | userId | 用户积分变更事件 |
| novel | novelId | 小说变更事件 |
| creditHistory | uuid | 积分历史记录 |

---

## 6. Event 模块 vs 原 blockchain

| 特性 | 原 blockchain | 新 Event 模块 |
|------|---------------|---------------|
| 持久化 | ❌ (内存) | ✅ MongoDB |
| 启动恢复 | 需手动 LoadFromFile | ✅ 自动从 MongoDB 加载 |
| 状态读取 | O(n) 扫描 | ✅ O(1) 从最新 event |
| 事件订阅 | ❌ | ✅ 内存 channel |
| SSE 推送 | 需要额外实现 | ✅ 内置 Subscribe |
| 额外进程 | ❌ | ❌ (复用 MongoDB) |

---

## 7. 使用示例

### 7.1 写入事件

```go
eventSvc := eventstore.NewEventService()

err := eventSvc.AppendEvent(
    context.Background(),
    "userCredit",
    "user123",
    "UserCreditRecharged",
    map[string]interface{}{
        "userId":  "user123",
        "amount":  100,
        "balance": 150,
    },
)
```

### 7.2 读取最新状态

```go
state, err := eventSvc.GetLatestState(
    context.Background(),
    "userCredit",
    "user123",
)
```

### 7.3 订阅事件变更

```go
ch, cleanup := eventSvc.Subscribe("userCredit")
defer cleanup()

for event := range ch {
    fmt.Printf("Event: %s, Data: %s\n", event.Type, string(event.Data))
}
```

---

## 8. 文件清单

```
token_blockchain/
├── eventstore/
│   └── client.go          # EventService 实现
├── database/
│   ├── models.go          # 新增 EventDocument
│   └── mongodb.go        # 新增 Event CRUD 操作
├── blockchain/            # (保留但不再使用)
│   └── blockchain.go
└── docs/
    └── event-module-solution.md  # 本文档
```

---

## 9. 待完成

- [x] 改造 service 层，全部使用 AppendEvent() 记录事件
- [ ] 删除或归档 blockchain/ 目录
- [ ] 添加 SSE 端点 `/api/v1/events/listen`
- [ ] 前端接入 SSE 实时接收事件
- [ ] 测试完整的数据流

---

## 10. 参考

- Event Sourcing 模式: https://martinfowler.com/eaaDev/EventSourcing.html
- MongoDB Change Streams: https://docs.mongodb.com/manual/changeStreams/