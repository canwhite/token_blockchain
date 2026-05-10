# 数据流对比：Fabric vs Blockchain

## 1. Fabric (ProChainRM) 数据流

```
┌─────────────────────────────────────────────────────────────────────┐
│                         写操作流程                                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  客户端 ────▶ SubmitTransaction ────▶ Fabric Chaincode 执行         │
│                                        │                            │
│                                        ▼                            │
│                               生成 ChaincodeEvent                    │
│                                        │                            │
│                                        ▼                            │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ EventService (独立后台 goroutine)                          │   │
│  │     │                                                        │   │
│  │     ▼                                                        │   │
│  │  network.ChaincodeEvents() ← 持续监听 channel               │   │
│  │     │                                                        │   │
│  │     ▼                                                        │   │
│  │  processEventAndSyncToMongoDB()                            │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                        │                            │
│                                        ▼                            │
│                               MongoDB (最终数据存储)                  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**特点**：
- EventService 是**独立后台进程**，持续运行
- 链码执行完成后自动触发事件
- MongoDB 是唯一真实数据源
- 服务重启后数据不丢失（MongoDB 持久化）

---

## 2. 我们 Blockchain 数据流（当前实现）

```
┌─────────────────────────────────────────────────────────────────────┐
│                         写操作流程                                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  客户端 ────▶ API Handler ────▶ Service Layer                        │
│                                        │                            │
│                                        ▼                            │
│                              blockchain.Write()                      │
│                                   │                                  │
│                    ┌──────────────┴──────────────┐                  │
│                    ▼                              ▼                  │
│              写入内存 Blockchain            可能失败                 │
│                    │                              │                  │
│                    ▼                              ▼                  │
│             MongoDB.Upsert()              (错误未处理)              │
│                    │                                                 │
│                    ▼                                                 │
│              同步完成                                                 │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**特点**：
- **同步写入**，不是事件驱动
- blockchain 是**内存存储**，服务重启后丢失
- MongoDB 同步**可能失败**但无重试机制
- 没有独立的同步后台进程

---

## 3. 读操作流程对比

### Fabric 读操作
```
客户端 ────▶ EvaluateTransaction ────▶ Fabric World State
                                        │
                                        ▼
                                  返回结果（已验证）
```

### Blockchain 读操作
```
客户端 ────▶ GetUserCredit()
                  │
                  ├─▶ MongoDB 查询（优先）
                  │       │
                  │       ├── 命中 ──▶ 返回
                  │       │
                  │       └── 未命中 ──▶ 区块链查询
                  │                       │
                  │                       ▼
                  │                 回填 MongoDB
                  │
                  └─▶ 直接区块链查询
```

---

## 4. 新用户首次访问流程

### Fabric (正常)
```
1. GET /api/v1/users/:id → 404 (不存在)
2. POST /api/v1/users → CreateUserCredit
3. SubmitTransaction → 链码执行 → EventService 监听 → 写入 MongoDB
4. 下次 GET → MongoDB 返回用户数据
```

### Blockchain (当前)
```
1. GET /api/v1/users/:id → 404
   │
   ├─ MongoDB 查询 → 不存在
   │
   └─ 区块链查询 → 不存在 (内存为空，服务重启后丢失)
                     │
                     ▼
                  返回 404

2. POST /api/v1/users → 创建
   │
   ├─ blockchain.Write() → 写入内存成功
   │
   └─ MongoDB.CreateUserCredit() → 可能成功/失败

3. 问题：
   - 服务重启后 blockchain 内存为空
   - MongoDB 同步可能失败但无感知
```

---

## 5. 服务重启后的差异

### Fabric
```
服务重启 ──▶ MongoDB 数据完整 ──▶ 正常使用
```

### Blockchain（问题所在）
```
服务重启 ──▶ blockchain 内存为空
              │
              ├─ MongoDB 有数据 → 可以回填（如果有回填逻辑）
              │
              └─ MongoDB 无数据 → 所有数据丢失
```

---

## 6. 核心问题总结

| 问题 | 原因 | 影响 |
|------|------|------|
| 数据丢失 | blockchain 存储在内存，无持久化 | 服务重启后用户数据消失 |
| 同步失败无感知 | MongoDB 写入失败未重试 | blockchain 和 MongoDB 数据不一致 |
| 无事件机制 | 我们是轮询，Fabric 是事件推送 | 实时性差，无法监听特定事件 |
| 无独立同步进程 | 依赖 API 同步写入 | 漏写风险 |

---

## 7. 改进方向

### 方案 A：持久化 blockchain 到文件
- 启动时从文件加载区块链数据
- 写入时同时更新文件
- 保持区块链结构（hash链）

### 方案 B：MongoDB 作为主数据源
- 去掉 in-memory blockchain
- MongoDB 是唯一真实源
- 简化架构，但失去区块链特性

### 方案 C：添加独立同步进程
- 类似 EventService 的后台 goroutine
- 监听区块链变化，同步到 MongoDB
- 需要改造 blockchain 为事件驱动