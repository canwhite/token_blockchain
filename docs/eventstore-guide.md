# EventStore 事件溯源系统详解

## 1. 什么是 EventStore？

**类比**：想象你有一个**账本**，每一笔收入和支出都按时间顺序记录下来。查看当前余额时，只需要把最后一笔记录读出来就行了。

**EventStore 就是这样一个"账本"**，它记录系统中发生的每一次"事件"（创建用户、充值、消费等）。

---

## 2. 核心概念

| 概念 | 解释 |
|------|------|
| **Stream（流）** | 类似于"话题"或"分类"。比如 `userCredit` 是用户积分的流，`novel` 是小说的流 |
| **Event（事件）** | 一次操作，比如"用户积分被创建"、"用户充值了10个token" |
| **EventData** | 内存中的事件表示（JSON 格式） |
| **EventDocument** | MongoDB 存储的事件文档 |
| **Version（版本）** | 事件的序号，用来保证顺序和检测遗漏 |
| **Emit** | 手动调用 AppendEvent 写入事件 |
| **Notify** | AppendEvent 内部自动触发的异步通知 |

---

## 3. Emit vs Notify 触发机制

这是两个**独立**的动作：

| 动作 | 触发方式 | 调用者 | 说明 |
|------|----------|--------|------|
| **Emit (AppendEvent)** | **手动调用** | Service 层 | 每次写操作时显式调用 |
| **Notify (notifySubscribers)** | **自动触发** | AppendEvent 内部 | goroutine 异步，不阻塞 |

### 代码位置

```go
// eventstore/client.go:69-111
func (s *EventService) AppendEvent(...) error {
    // ...写入 MongoDB...

    // Emit 是显式调用，这里是手动调的地方
    // s.eventSvc.AppendEvent(ctx, "userCredit", userId, "UserCreditCreated", uc)

    // Notify 是 AppendEvent 的副作用，自动触发
    go s.notifySubscribers(streamName, eventData)  // 第109行
}
```

### 流程图

```
┌─────────────────────────────────────────────────────────────────────┐
│  AppendEvent() 被 Service 手动调用                                   │
│                                                                     │
│    ├── 1. JSON 序列化 data                                          │
│    ├── 2. 查询 version                                              │
│    ├── 3. 写入 MongoDB (同步，阻塞)                                   │
│    └── 4. go notifySubscribers() (异步，不阻塞)                       │
│              │                                                      │
│              └── 通知所有 Subscribe() 注册的 channel                │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 4. 完整数据流

### 写入流程（以创建用户为例）

```
用户请求: POST /api/v1/users
    │
    ▼
api/server.go: createUserCredit()
    │
    ▼
service/user_service.go: CreateUserCredit(&uc)
    │
    ├── 1. 验证 userId 不为空
    ├── 2. 生成 UUID（如果 ID 为空）
    │
    ▼
    3. 调用 eventSvc.AppendEvent()
    │
    ▼
eventstore/client.go: AppendEvent()
    │
    ├── 3.1 JSON 序列化 data → jsonData
    ├── 3.2 获取当前版本号 getEventCount()
    ├── 3.3 创建 EventDocument
    │       - ID: uuid.New()
    │       - Type: "UserCreditCreated"
    │       - StreamName: "userCredit"
    │       - StreamID: userId
    │       - Data: jsonData ([]byte)
    │       - Version: 0
    │       - CreatedAt: time.Now()
    │
    ▼
    4. 调用 database.CreateEvent(ctx, eventDoc)
    │
    ▼
database/mongodb.go: CreateEvent()
    │
    └── 5. InsertOne() 写入 events 表
            │
            ▼
    ┌─────────────────────────────────────────┐
    │ MongoDB events collection               │
    │ {                                       │
    │   "_id": "event-uuid-123",              │
    │   "type": "UserCreditCreated",          │
    │   "streamName": "userCredit",           │
    │   "streamId": "69ff438c6e...",          │
    │   "data": "{...}",                      │
    │   "version": 0,                         │
    │   "createdAt": "2026-05-10T..."         │
    │ }                                       │
    └─────────────────────────────────────────┘
            │
            ▼ 异步（goroutine）
    6. go notifySubscribers()
            │
            ▼
    7. 通知所有订阅者
```

### 查询流程

```
GetUserCredit(userId)
    │
    ├──→ 先查 MongoDB user_credits 表（快速路径）
    │      ├── 找到了 → 直接返回
    │      └── 没找到 → 继续
    │
    └──→ GetLatestState("userCredit", userId)
           │
           └── 去 events 表查该 userId 的最后一条事件
                  取出 Data，反序列化返回
```

---

## 5. 核心函数说明

### AppendEvent - 追加事件（手动触发）

```go
func (s *EventService) AppendEvent(
    ctx context.Context,
    streamName string,       // 流名称（如 "userCredit"）
    streamID string,          // 实体ID（如用户ID）
    eventType string,        // 事件类型（如 "UserCreditCreated"）
    data interface{},        // 事件数据（任意结构体）
) error
```

**执行步骤：**
1. 加写锁（mu.Lock）
2. 将 data 序列化为 JSON
3. 查询当前事件数量作为 version
4. 创建 EventDocument
5. 写入 MongoDB 的 events 表
6. 解锁
7. 异步通知订阅者（goroutine）

### notifySubscribers - 通知订阅者（自动触发）

```go
func (s *EventService) notifySubscribers(streamName string, event EventData)
```

- **不是手动调用**，是 AppendEvent 的副作用
- 使用 `goroutine` 异步执行，不阻塞主流程
- 遍历 `subs` map，找到匹配的 streamName
- 向每个订阅者的 channel 发送事件（非阻塞）

### Subscribe - 订阅事件流

```go
func (s *EventService) Subscribe(streamName string) (<-chan EventData, func())
```

**返回：**
- `chan EventData` - 事件通道，接收该流的新事件
- `func()` - 取消订阅的清理函数

**使用示例：**
```go
// 订阅用户积分的事件
ch, cleanup := eventSvc.Subscribe("userCredit")

// 在协程中接收事件
go func() {
    for event := range ch {
        fmt.Printf("收到事件: Type=%s, StreamID=%s\n", event.Type, event.StreamID)
    }
}()

// 当不再需要订阅时，调用 cleanup()
cleanup()
```

### Subscribe 调用时机

**当前状态：`Subscribe()` 函数已定义，但项目中暂未使用。**

| 函数 | 调用状态 | 说明 |
|------|----------|------|
| `AppendEvent` | ✅ 被调用 | Service 层手动调用 |
| `notifySubscribers` | ✅ 被调用 | AppendEvent 内部自动触发（goroutine） |
| `Subscribe` | ❌ 未使用 | 定义了函数，但无调用方 |

**设计意图：** `Subscribe` 是给需要**实时接收事件通知**的场景准备的，比如：

| 潜在场景 | 说明 |
|----------|------|
| **WebSocket 实时推送** | 前端 WebSocket 订阅，事件来了实时推送给客户端 |
| **缓存失效** | 事件来了自动清除相关缓存 |
| **后台任务触发** | 事件来了触发异步任务（如发邮件、发通知） |

**调用流程：**

```
1. 某处调用 Subscribe("userCredit")  → 创建 channel，注册到 subs map
2. AppendEvent 触发                   → 自动 notifySubscribers
3. notifySubscribers 找到对应 channel → 发送事件到 channel
4. 调用方从 channel 接收事件          → 处理业务逻辑
5. 调用 cleanup()                     → 取消订阅，关闭 channel
```

**使用示例（设计意图）：**

```go
// 在 WebSocket 连接建立时订阅
func handleWebSocket(conn *websocket.Conn) {
    ch, cleanup := eventSvc.Subscribe("userCredit")

    // 在协程中接收事件并推送给 WebSocket 客户端
    go func() {
        for event := range ch {
            conn.WriteJSON(event)  // 实时推送
        }
    }()

    // WebSocket 断开时清理
    conn.Close()
    cleanup()
}
```

### GetLatestState - 获取最新状态

```go
func (s *EventService) GetLatestState(
    ctx context.Context,
    streamName string,
    streamID string,
) (interface{}, error)
```

**执行步骤：**
1. 调用 `database.GetLatestEvent(streamName, streamID)`
2. 从 events 表取出该 streamID 的最新事件（按 version 降序排列）
3. 解析事件 Data 字段为 JSON
4. 返回

---

## 6. 数据结构

### EventData（内存中的事件表示）

```go
type EventData struct {
    ID         string          // 事件唯一ID
    Type       string          // 事件类型（UserCreditCreated, Consume, Recharge等）
    Data       json.RawMessage // 事件数据（JSON格式的原始字节）
    StreamName string          // 属于哪个流
    StreamID   string          // 具体是哪个实体的ID
    Version    int64           // 版本号（从0开始递增）
    CreatedAt  time.Time       // 发生时间
}
```

**为什么 Data 用 json.RawMessage？**
- 本质是 `[]byte`，存储原始 JSON 字节
- 存入时是什么格式，取出还是什么格式
- 等需要用的时候再 Unmarshal，更灵活高效

### EventDocument（MongoDB 存储）

```go
type EventDocument struct {
    ID         string    `bson:"_id"`       // MongoDB 文档ID
    Type       string    `bson:"type"`      // 事件类型
    StreamName string    `bson:"streamName"`// 流名称
    StreamID   string    `bson:"streamId"`  // 实体ID
    Data       []byte    `bson:"data"`      // 事件数据（JSON字节）
    Version    int64     `bson:"version"`   // 版本号
    CreatedAt  time.Time `bson:"createdAt"` // 创建时间
}
```

### 事件类型常量

```go
const (
    EventUserCreditCreated   = "UserCreditCreated"   // 用户积分创建
    EventUserCreditUpdated   = "UserCreditUpdated"   // 用户积分更新
    EventUserCreditRecharged = "UserCreditRecharged" // 用户充值
    EventUserCreditConsumed = "UserCreditConsumed"   // 用户消费
    EventNovelCreated       = "NovelCreated"          // 小说的创建
    EventNovelUpdated       = "NovelUpdated"          // 小说的更新
    EventNovelDeleted       = "NovelDeleted"          // 小说的删除
)
```

### 流名称常量

```go
const (
    StreamUserCredit = "userCredit" // 用户积分流
    StreamNovel     = "novel"       // 小说的流
)
```

---

## 7. MongoDB 存储结构

### events 表

```json
{
    "_id": "event-uuid-123",
    "type": "UserCreditCreated",
    "streamName": "userCredit",
    "streamId": "user-id-xxx",
    "data": "{\"userId\":\"xxx\",\"credit\":100,...}",
    "version": 0,
    "createdAt": "2026-05-10T12:00:00Z"
}
```

### 索引

- `_id` - 主键索引（隐式）
- `streamName + streamId` - 联合索引，用于快速查询某实体的所有事件
- `version` - 用于排序

---

## 8. 为什么使用 EventStore？

### 传统方式 vs EventStore 方式

| 特性 | 传统方式 | EventStore 方式 |
|------|----------|-----------------|
| 存储内容 | 最终状态（如 credit=100） | 所有历史事件 |
| 数据丢失 | 丢了就没了 | 有事件就能重建 |
| 审计追踪 | 难以追溯"怎么变成这样的" | 事件就是完整的操作日志 |
| 多服务同步 | 共享数据库，耦合紧 | 事件驱动，松耦合 |
| 故障恢复 | 需要备份+日志 | 直接从事件重建状态 |

### EventStore 的优势

1. **事件不可变，只能追加** - 保证数据完整性
2. **完整的历史** - 可以追溯任何时间点的状态
3. **版本号控制** - 可以检测事件是否有遗漏
4. **易于调试** - 任何问题都可以通过重放事件重现
5. **解耦** - 生产者和消费者通过事件交互，不直接依赖
6. **实时通知** - 通过 Subscribe 机制可以实时接收事件通知

---

## 9. 关键调用场景

### 触发 AppendEvent 的场景

| 操作 | API | 触发函数 |
|------|-----|---------|
| 创建用户 | `POST /users` | `AppendEvent(...EventUserCreditCreated...)` |
| 充值 | `POST /users/recharge` | `AppendEvent(...EventUserCreditRecharged...)` |
| 消费token | `POST /users/:id/consume-token` | `AppendEvent(...EventUserCreditConsumed...)` |
| 创建小说 | `POST /novels` | `AppendEvent(...EventNovelCreated...)` |

### 触发 GetLatestState 的场景

| 操作 | API | 说明 |
|------|-----|------|
| 获取单个用户 | `GET /users/:id` | MongoDB 查不到时回退到 EventStore |
| 获取所有用户 | `GET /users` | MongoDB 为空时回退到 EventStore |
| 获取单个小說 | `GET /novels/:id` | MongoDB 查不到时回退到 EventStore |

---

## 10. EventStore 与 Blockchain 对比

### 共同点：都是 Event Sourcing 模式

| 理念 | 说明 |
|------|------|
| **存事件而非状态** | 记录"发生了什么"，而不是"现在是什么" |
| **追加模式** | 只追加不修改，历史永不丢失 |
| **状态可重建** | 从事件可以还原任意时间点的状态 |

### 架构对比

```
┌─────────────────────────────────────────────────────────────────────┐
│                    EventStore（当前项目）                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  events 表（扁平结构）                                              │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐                              │
│  │ Event 0 │→ │ Event 1 │→ │ Event 2 │→ ...                         │
│  └─────────┘  └─────────┘  └─────────┘                              │
│       │            │            │                                    │
│  version=0     version=1     version=2                              │
│                                                                     │
│  查询：直接找 version 最高的                                        │
│  防篡改：依赖 MongoDB 权限控制                                       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                    Blockchain（novel-resource-management）           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Chain of Blocks（链式结构）                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                          │
│  │ Block 0  │→ │ Block 1  │→ │ Block 2  │→ ...                    │
│  │(创世区块)│  │          │  │          │                          │
│  └──────────┘  └──────────┘  └──────────┘                          │
│       │            │            │                                     │
│  prev:null    prev:hash0   prev:hash1                              │
│  hash:0       hash:1       hash:2                                  │
│                                                                     │
│  查询：重放所有 blocks 或维护快照                                    │
│  防篡改：每个 block 包含前一个 hash（密码学保证）                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 核心区别

| | EventStore（我们的） | Blockchain（novel-resource-management） |
|---|---|---|
| **存储结构** | 扁平的 events 表，按 version 排序 | 链式 blocks，每个 block 指向前一个 hash |
| **核心逻辑** | 记录事件 → 存储 → 可查询重建状态 | 记录事件 → 打包成 block → 链接成链 |
| **不可变性** | 通过 MongoDB 数据库权限控制 | 通过 Hash 链保证（篡改会断开被发现） |
| **状态重建** | 直接查 events 表，取 version 最高的 | 重放所有 blocks，或维护本地快照 |
| **查询性能** | 直接查快（无需重放） | 需要重放（慢）或维护快照（复杂） |
| **去中心化** | 否（依赖中心化数据库） | 是（分布式节点共同维护） |
| **适用场景** | 内部系统、日志、事件流 | 需要信任、防篡改、多方共享 |

### 关键代码对比

**EventStore 存储：**
```go
// 直接追加事件到 events 表
eventDoc := &database.EventDocument{
    ID:        uuid.New().String(),
    Type:      eventType,
    StreamName: streamName,
    StreamID:   streamID,
    Data:       jsonData,
    Version:    version,  // 简单的序号
}
database.CreateEvent(ctx, eventDoc)
```

**Blockchain 存储：**
```go
// 打包成 block，包含前一个 hash
block := &Block{
    Index:     index,
    Timestamp: timestamp,
    Events:    events,
    PrevHash:  previousBlock.Hash,  // 链式链接
    Hash:      calculateHash(...),   // 当前 block 的指纹
}
// block 存入 chain
chain.AddBlock(block)
```

### 一句话总结

- **EventStore**：轻量级事件溯源，依赖数据库的不可变性
- **Blockchain**：加强版事件溯源，用 hash 链让篡改无处遁形

两者本质都是 **"只追加的事件日志"**，只是防篡改机制和存储结构不同。

---

## 11. 文件位置

- `eventstore/client.go` - EventService 的实现（246行）
- `database/mongodb.go` - MongoDB 相关的辅助函数（如 `CreateEvent`、`GetLatestEvent` 等）
- `database/models.go` - 数据模型定义（如 `EventDocument`）

---

## 12. 一句话总结

1. **触发写入**：Service 层每次写操作时**手动调用** `AppendEvent`
2. **异步通知**：`AppendEvent` 内部**自动**通过 goroutine 调用 `notifySubscribers`
3. **查询降级**：先查 MongoDB，**查不到**才用 `GetLatestState` 从 events 反推状态

这就是 **Event Sourcing（事件溯源）** 模式！
