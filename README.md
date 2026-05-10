# token_blockchain

## 项目说明

这是一个**使用区块链思维的轻量级事件溯源系统**，用于数据备份和问题追踪。

**核心特点**：实现了类似区块链的 Event Sourcing（事件溯源）逻辑，但完全不使用区块链技术。

---

## 为什么叫 "token_blockchain" 但没有 blockchain？

### 回答

| 对比项 | blockchain（区块链） | token_blockchain（本项目） |
|--------|----------------------|---------------------------|
| **存储结构** | 链式 blocks（hash 链接） | 扁平的 events 表 |
| **防篡改** | Hash 链（密码学保证） | MongoDB 权限控制 |
| **去中心化** | 分布式多节点 | 中心化单节点 |
| **共识机制** | POW/POS 等 | 无 |
| **事件溯源** | ✅ | ✅ |
| **追加模式** | ✅ | ✅ |
| **状态可重建** | ✅ | ✅ |

### 结论

**"blockchain" 在这里是一种思维模式，不是技术实现。**

本项目借鉴了区块链的 **Event Sourcing（事件溯源）** 思想：
- 每次操作都记录为一个**不可变事件**
- 通过 **stream + version** 保证事件顺序
- 可以从事件历史**重建任意时间点的状态**

但为了轻量化和实用性，去掉了：
- 密码学 hash 链（不必要的复杂度）
- 分布式共识机制（单节点足够）
- P2P 网络（自己用不需要）

---

## 核心概念

### EventStore（事件溯源）

```
┌─────────────────────────────────────────────────────────────┐
│                        events 表                            │
├─────────────────────────────────────────────────────────────┤
│ {                                                           │
│   "streamName": "userCredit",                              │
│   "streamId": "user-123",                                  │
│   "type": "UserCreditCreated",                             │
│   "data": {...},                                           │
│   "version": 0                                             │
│ }                                                          │
│ {                                                           │
│   "streamName": "userCredit",                              │
│   "streamId": "user-123",                                  │
│   "type": "UserCreditConsumed",                            │
│   "data": {...},                                           │
│   "version": 1                                             │
│ }                                                          │
└─────────────────────────────────────────────────────────────┘
```

### 核心机制

| 机制 | 说明 |
|------|------|
| **Stream（流）** | 事件分组，如 `userCredit`、`novel` |
| **Event（事件）** | 每次操作记录为一个事件，如 `UserCreditCreated` |
| **Version（版本）** | 事件序号，保证顺序 |
| **AppendEvent** | 手动调用，写入事件 |
| **Subscribe** | 订阅事件流，实时接收通知（设计预留） |

### 数据流

```
写操作：Service → AppendEvent → events 表 + 自动通知订阅者
读操作：先查 MongoDB 表 → 查不到则从 events 重建状态
```

详细说明请参阅 [docs/eventstore-guide.md](docs/eventstore-guide.md)。

---

## 技术栈

- **Gin** - HTTP 框架
- **MongoDB** - 数据库（存储 events 和业务数据）
- **EventStore** - 自研事件溯源实现

---

## 文件结构

```
token_blockchain/
├── api/              # HTTP 接口层
├── service/          # 业务逻辑层
├── database/         # MongoDB 操作
├── eventstore/       # 事件溯源核心（EventService）
├── middleware/       # 中间件（RSA 加解密）
├── docs/            # 文档
│   └── eventstore-guide.md  # EventStore 详细说明
├── main.go          # 入口
└── .env            # 配置
```

---

## 运行

```bash
go build -o token_blockchain .
./token_blockchain
```

服务默认端口 `8080`，访问 `http://localhost:8080/health` 检查状态。
