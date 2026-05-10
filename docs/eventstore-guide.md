# EventStore 事件溯源系统讲解

## 1. 什么是 EventStore？

**类比**：想象你有一个**账本**，每一笔收入和支出都按时间顺序记录下来。查看当前余额时，只需要把最后一笔记录读出来就行了。

**EventStore 就是这样一个"账本"**，它记录系统中发生的每一次"事件"（创建用户、充值、消费等）。

---

## 2. 核心概念

| 概念 | 解释 |
|------|------|
| **Stream（流）** | 类似于"话题"或"分类"。比如 `userCredit` 是用户积分的流，`novel` 是小说的流 |
| **Event（事件）** | 一次操作，比如"用户积分被创建"、"用户充值了10个token" |
| **Version（版本）** | 事件的序号，用来保证顺序和检测遗漏 |

---

## 3. 数据结构

### EventData（内存中的事件表示）

```go
type EventData struct {
    ID         string          // 事件唯一ID
    Type       string          // 事件类型（UserCreditCreated, Consume, Recharge等）
    Data       json.RawMessage // 事件数据（JSON格式）
    StreamName string          // 属于哪个流
    StreamID   string          // 具体是哪个实体的ID
    Version    int64           // 版本号（从0开始递增）
    CreatedAt  time.Time       // 发生时间
}
```

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
    EventUserCreditConsumed  = "UserCreditConsumed"   // 用户消费
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

## 4. 数据流图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           写入流程（AppendEvent）                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. 你调用 API: POST /api/v1/users                                  │
│     ↓                                                               │
│  2. createUserCredit() 处理函数收到请求                              │
│     ↓                                                               │
│  3. 调用 userService.CreateUserCredit(&uc)                          │
│     ↓                                                               │
│  4. UserService 内部：                                              │
│     a) 生成 UUID 作为事件 ID                                        │
│     b) 调用 eventSvc.AppendEvent() 记录事件                         │
│     ↓                                                               │
│  5. AppendEvent() 做三件事：                                         │
│     - 把事件数据变成 JSON                                            │
│     - 查询当前事件数量，算出版本号（version = 事件数量）              │
│     - 把事件写入 MongoDB 的 events 表                                │
│     ↓                                                               │
│  6. 同时调用 database.CreateUserCredit()                            │
│     在 user_credits 表也写入一份（为了快速查询）                     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                           读取流程（GetLatestState）                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. 你调用 API: GET /api/v1/users/xxx                               │
│     ↓                                                               │
│  2. userService.GetUserCredit(xxx) 被调用                           │
│     ↓                                                               │
│  3. 先查 MongoDB 的 user_credits 表                                 │
│     - 如果找到了，直接返回（快速路径）                                │
│     - 如果没找到，继续往下                                            │
│     ↓                                                               │
│  4. 如果 MongoDB 里没有，去 EventStore 查                            │
│     - 调用 eventSvc.GetLatestState("userCredit", "user-id")          │
│     ↓                                                               │
│  5. GetLatestState() 做的事：                                        │
│     - 去 events 表查：这个用户的最后一条事件                         │
│     - 取出事件里的 Data                                              │
│     - 返回给调用者                                                   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 5. 核心函数说明

### AppendEvent - 追加事件

```go
func (s *EventService) AppendEvent(
    ctx context.Context,
    streamName string,       // 流名称（如 "userCredit"）
    streamID string,        // 实体ID（如用户ID）
    eventType string,        // 事件类型（如 "UserCreditCreated"）
    data interface{},        // 事件数据（任意结构体）
) error
```

**执行步骤：**
1. 加写锁
2. 将 data 序列化为 JSON
3. 查询当前事件数量作为 version
4. 创建 EventDocument
5. 写入 MongoDB 的 events 表
6. 解锁
7. 异步通知订阅者

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

### GetAllLatestStates - 获取所有最新状态

```go
func (s *EventService) GetAllLatestStates(
    ctx context.Context,
    streamName string,
) ([]interface{}, error)
```

**执行步骤：**
1. 调用 `database.GetLatestEventsByStream(streamName)`
2. 使用 MongoDB 聚合管道，获取每个 streamID 的最新事件
3. 解析每个事件的 Data
4. 返回所有状态的数组

### Subscribe - 订阅事件流

```go
func (s *EventService) Subscribe(streamName string) (<-chan EventData, func())
```

**返回：**
- `chan EventData` - 事件通道，接收该流的新事件
- `func()` - 取消订阅的清理函数

---

## 6. 为什么使用 EventStore？

### 传统方式 vs EventStore 方式

| 特性 | 传统方式 | EventStore 方式 |
|------|----------|-----------------|
| 存储内容 | 最终状态（如 credit=100） | 所有历史事件 |
| 数据丢失 | 丢了就没了 | 有事件就能重建 |
| 审计追踪 | 难以追溯"怎么变成这样的" | 事件就是完整的操作日志 |
| 多服务同步 | 共享数据库，耦合紧 | 事件驱动，松耦合 |
| 故障恢复 | 需要备份+日志 | 直接从事件重建状态 |
| 性能 | 每次更新都要读写数据库 | 写入快，读取可从快照获取 |

### EventStore 的优势

1. **事件不可变，只能追加** - 保证数据完整性
2. **完整的历史** - 可以追溯任何时间点的状态
3. **版本号控制** - 可以检测事件是否有遗漏
4. **易于调试** - 任何问题都可以通过重放事件重现
5. **解耦** - 生产者和消费者通过事件交互，不直接依赖

---

## 7. MongoDB 存储结构

### events 表

```
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

### 关键查询

```go
// 查询某实体的所有事件（按版本升序）
{streamName: "userCredit", streamId: "xxx"} sort by {version: 1}

// 获取某实体的最新事件
{streamName: "userCredit", streamId: "xxx"} sort by {version: -1} limit 1

// 聚合获取所有实体的最新事件
$match: {streamName: "userCredit"}
$sort: {version: -1}
$group: {_id: "$streamId", ...}
```

---

## 8. 订阅机制（发布-订阅模式）

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

**通知机制（非阻塞）：**
- 使用 `select` + `default`，如果通道满了不会阻塞，直接丢弃
- 异步通知，不影响主流程性能

---

## 9. 简单总结

1. **每一次操作都记一个事件** → AppendEvent
2. **想查当前状态** → 先看 MongoDB（快速），没有就读 EventStore 最新事件
3. **事件不可变，只能追加** → 保证数据完整性
4. **通过 Version 保证顺序** → 可以检测事件是否有遗漏

这就是 **Event Sourcing（事件溯源）** 模式！

---

## 10. 文件位置

- `eventstore/client.go` - EventService 的实现
- `database/mongodb.go` - MongoDB 相关的辅助函数（如 `CreateEvent`、`GetLatestEvent` 等）
- `database/models.go` - 数据模型定义（如 `EventDocument`）
