# State 和 Event 变更实现总结

## 核心概念

| 概念 | 说明 |
|------|------|
| **State** | 当前业务数据（UserCredit、Novel 等），存储在 MongoDB 业务集合 |
| **Event** | 状态变更记录，存储在 MongoDB `events` 集合 |

## 两种数据流

### 1. 写入流程（Write Path）

```
Service.Recharge(userId, 100)
    │
    ├─► eventSvc.AppendEvent()
    │         │
    │         ├─► 序列化 event 数据为 JSON
    │         ├─► 计算 version（事件总数）
    │         ├─► 创建 EventDocument
    │         └─► database.CreateEvent() → MongoDB events 集合
    │
    └─► database.UpsertUserCredit(uc)
              │
              └─► MongoDB user_credits 集合（最新 state）
```

**写入两个地方：**
- `events` 集合 - 记录完整变更历史（不可变）
- `user_credits` 集合 - 存储当前 state（可覆盖）

### 2. 读取流程（Read Path）

```
Service.GetUserCredit(userId)
    │
    ├─► database.GetUserCredit(userId)
    │         │
    │         └─► MongoDB user_credits 集合 → 直接返回 state
    │
    └─► （如果 MongoDB 无数据）
          eventSvc.GetLatestState()
                    │
                    └─► MongoDB events 集合
                          按 streamName-streamID 查询
                          取 version 最大的 EventDocument
                          解析 data 字段为 state
```

## Event Document 结构

```go
type EventDocument struct {
    ID         string    // UUID
    Type       string    // "UserCreditRecharged"
    StreamName string    // "userCredit"
    StreamID   string    // "userId-xxx"
    Data       []byte    // JSON 序列化的完整 state
    Version    int64     // 版本号（递增）
    CreatedAt  time.Time
}
```

## Stream 命名

```
streamName-streamID 组成唯一标识

userCredit-69ff438c6e3717d077de9996
novel-a1b2c3d4-e5f6
```

## 状态重建（State Rebuild）

服务重启后，state 从两个来源恢复：

1. **优先**：MongoDB 业务集合（`user_credits`、`novels`）
2. **回退**：从 `events` 集合按 stream 读取所有事件，按 version 顺序重放

## 事件订阅（Subscription）

```
client
  │
  ▼
eventSvc.Subscribe("userCredit")
  │
  ├─► 返回 <-chan EventData
  │
  └─► AppendEvent 时
        │
        └─► notifySubscribers()
              │
              └─► 推送到所有订阅的 channel
```

SSE 端点 `/api/v1/events/listen` 尚未实现（待办）。

## 关键特性

| 特性 | 实现 |
|------|------|
| 持久化 | ✅ MongoDB events 集合 |
| 状态重建 | ✅ GetLatestState 从最新 event 恢复 |
| 事件历史 | ✅ events 集合保留完整变更链 |
| 实时订阅 | ✅ 内存 channel（仅内存，重启丢失） |
| 幂等写 | ✅ version 递增，event 只追加不修改 |

## 相关文件

```
token_blockchain/
├── eventstore/
│   └── client.go          # EventService 实现
├── database/
│   ├── models.go          # EventDocument 模型
│   └── mongodb.go         # Event CRUD 操作
├── service/
│   ├── user_service.go    # 用户积分服务（使用 eventstore）
│   └── novel_service.go   # 小说服务（使用 eventstore）
└── docs/
    ├── event-module-solution.md    # 完整方案文档
    └── state-event-summary.md      # 本文档
```

## 待办

- [ ] 删除或归档 blockchain/ 目录
- [ ] 添加 SSE 端点 `/api/v1/events/listen`
- [ ] 前端接入 SSE 实时接收事件
- [ ] 测试完整的数据流
