# 区块链底座改造实施计划

## 1. 项目背景

### 1.1 目标
将当前简单的 token_blockchain 项目改造为可用于小说资源管理的区块链底座，复刻 ProChainRM 原项目（基于 Hyperledger Fabric）的核心接口，但使用轻量级区块链替代 Fabric。

### 1.2 源项目核心功能
- **Novel 资源管理**: 小说的 CRUD 操作
- **UserCredit 积分管理**: 用户积分的 CRUD、充值、消费
- **CreditHistory 交易记录**: 所有积分变动的链上记录
- **RechargeRecord 充值记录**: 充值订单记录（幂等性保障）
- **User 用户**: 用户账户信息（MongoDB 存储）

### 1.3 三种链上数据类型
| 数据类型 | 说明 | 链码函数前缀 |
|----------|------|-------------|
| `novel` | 小说资源（作者、故事大纲、角色等） | CreateNovel, ReadNovel, UpdateNovel, DeleteNovel, GetAllNovels |
| `user_credit` | 用户积分（余额、充值、消费） | CreateUserCredit, ReadUserCredit, UpdateUserCredit, DeleteUserCredit, GetAllUserCredits |
| `credit_history` | 积分变动历史（充值、消费、奖励） | CreateCreditHistory |

---

## 2. 前提条件

### 2.1 架构决策
| 决策项 | 选择 | 说明 |
|--------|------|------|
| 存储策略 | **混合模式** | 区块链作为可信源，MongoDB 作为缓存/索引层 |
| Token 交易记录 | **链上记录** | 充值/消费均生成 CreditHistory 存储在链上 |
| 共识机制 | **单节点** | 当前为 MVP，暂不实现多节点共识 |

### 2.2 技术选型
| 组件 | 选型 | 版本 |
|------|------|------|
| 区块链 | 自研轻量级区块链 | - |
| HTTP 框架 | Gin | v1.10+ |
| MongoDB 驱动 | mongo-go-driver | v1.13+ |
| JSON 处理 | encoding/json | 标准库 |

### 2.3 数据流设计
```
┌─────────────────────────────────────────────────────────────────┐
│                         写操作流程                               │
├─────────────────────────────────────────────────────────────────┤
│  API Request                                                    │
│       │                                                          │
│       ▼                                                          │
│  Service Layer (业务逻辑验证)                                   │
│       │                                                          │
│       ▼                                                          │
│  Blockchain Layer (写入链上，返回 Block)                         │
│       │                                                          │
│       ▼                                                          │
│  MongoDB Layer (同步缓存)                                       │
│       │                                                          │
│       ▼                                                          │
│  Response                                                       │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                         读操作流程                               │
├─────────────────────────────────────────────────────────────────┤
│  API Request                                                    │
│       │                                                          │
│       ▼                                                          │
│  MongoDB Layer (优先查询缓存)                                    │
│       │                                                          │
│       ├─── 命中 ────▶ Response (直接返回)                        │
│       │                                                          │
│       └─── 未命中 ──▶ Blockchain Layer (查询链上)                │
│                           │                                     │
│                           ▼                                     │
│                       MongoDB Layer (回填缓存)                   │
│                           │                                     │
│                           ▼                                     │
│                       Response                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 3. 方案设计

### 3.1 区块链层改造

#### 3.1.1 新增 DataType 类型
```go
type DataType string

const (
    DataTypeNovel      DataType = "novel"
    DataTypeUserCredit DataType = "user_credit"
    DataTypeHistory    DataType = "credit_history"
)
```

#### 3.1.2 改造 Block 结构
```go
type Block struct {
    Index    int      `json:"index"`
    Time     string   `json:"time"`
    DataType DataType `json:"dataType"`
    Key      string   `json:"key"`      // 复合键: "{dataType}:{id}"
    Value    string   `json:"value"`    // JSON 序列化的业务数据
    Hash     string   `json:"hash"`
    PrevHash string   `json:"prevHash"`
}
```

#### 3.1.3 区块链索引设计
| Key 格式 | 示例 | 说明 |
|----------|------|------|
| `novel:{uuid}` | `novel:550e8400-e29b...` | 单个小说 |
| `user_credit:{userId}` | `user_credit:user123` | 单个用户积分 |
| `credit_history:{uuid}` | `credit_history:hist456` | 单条历史记录 |

#### 3.1.4 区块链层方法
```go
func Write(dataType DataType, key string, value string) (Block, error)
func Read(dataType DataType, key string) (string, error)
func ReadAll(dataType DataType) ([]string, error)
func Delete(dataType DataType, key string) error
func GetLatestBlock() Block
func CalculateHash(block Block) string
func IsBlockValid(newBlock, oldBlock Block) bool
func KeyExists(dataType DataType, key string) bool
func ReplaceChain(newBlocks []Block)
```

### 3.2 数据模型设计

#### 3.2.1 Novel (小说)
```go
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
```

#### 3.2.2 UserCredit (用户积分)
```go
type UserCredit struct {
    ID            string `json:"id" bson:"_id,omitempty"`
    UserID        string `json:"userId" bson:"userId"`
    Credit        int    `json:"credit" bson:"credit"`
    TotalUsed     int    `json:"totalUsed" bson:"totalUsed"`
    TotalRecharge int    `json:"totalRecharge" bson:"totalRecharge"`
    CreatedAt     string `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
    UpdatedAt     string `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}
```

#### 3.2.3 CreditHistory (积分历史)
```go
type CreditHistory struct {
    ID          string `json:"id" bson:"_id,omitempty"`
    UserID      string `json:"userId" bson:"userId"`
    Amount      int    `json:"amount" bson:"amount"`
    Type        string `json:"type" bson:"type"`  // "consume", "recharge", "reward"
    Description string `json:"description" bson:"description"`
    Timestamp   string `json:"timestamp" bson:"timestamp"`
    NovelID     string `json:"novelId,omitempty" bson:"novelId,omitempty"`
}
```

#### 3.2.4 RechargeRecord (充值记录)
```go
type RechargeRecord struct {
    ID          string `json:"id" bson:"_id,omitempty"`
    OrderSN     string `json:"orderSn" bson:"orderSn"`
    UserID      string `json:"userId" bson:"userId"`
    Amount      int    `json:"amount" bson:"amount"`
    Status      string `json:"status" bson:"status"`  // "pending", "completed", "failed"
    Description string `json:"description" bson:"description"`
    CreatedAt   string `json:"createdAt" bson:"createdAt"`
    UpdatedAt   string `json:"updatedAt" bson:"updatedAt"`
}
```

#### 3.2.5 User (用户) - MongoDB Only
```go
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
```

### 3.3 API 设计

#### 3.3.1 Novel 接口
| Method | Endpoint | Handler | 说明 |
|--------|----------|---------|------|
| GET | `/api/v1/novels` | `getAllNovels` | 获取所有小说 |
| GET | `/api/v1/novels/:id` | `getNovel` | 获取单个小说 |
| POST | `/api/v1/novels` | `createNovel` | 创建小说 |
| PUT | `/api/v1/novels/:id` | `updateNovel` | 更新小说 |
| DELETE | `/api/v1/novels/:id` | `deleteNovel` | 删除小说 |

#### 3.3.2 UserCredit 接口
| Method | Endpoint | Handler | 说明 |
|--------|----------|---------|------|
| GET | `/api/v1/users` | `getAllUserCredits` | 获取所有用户积分 |
| GET | `/api/v1/users/:id` | `getUserCredit` | 获取单个用户积分 |
| POST | `/api/v1/users` | `createUserCredit` | 创建用户积分 |
| PUT | `/api/v1/users/:id` | `updateUserCredit` | 更新用户积分 |
| DELETE | `/api/v1/users/:id` | `deleteUserCredit` | 删除用户积分 |
| POST | `/api/v1/users/recharge` | `rechargeUserTokens` | 充值积分（HMAC签名验证） |
| POST | `/api/v1/users/:id/consume` | `consumeUserToken` | 消费积分 |

### 3.4 Service 层业务逻辑

#### 3.4.1 NovelService
```go
type NovelService struct{}

func (s *NovelService) CreateNovel(novel *Novel) error
func (s *NovelService) GetNovel(id string) (*Novel, error)
func (s *NovelService) GetAllNovels() ([]*Novel, error)
func (s *NovelService) UpdateNovel(id string, novel *Novel) error
func (s *NovelService) DeleteNovel(id string) error
```

#### 3.4.2 UserService
```go
type UserService struct{}

func (s *UserService) CreateUserCredit(uc *UserCredit) error
func (s *UserService) GetUserCredit(userId string) (*UserCredit, error)
func (s *UserService) GetAllUserCredits() ([]*UserCredit, error)
func (s *UserService) UpdateUserCredit(userId string, uc *UserCredit) error
func (s *UserService) DeleteUserCredit(userId string) error
func (s *UserService) Recharge(userId string, amount int, desc string) (*UserCredit, error)
func (s *UserService) Consume(userId string, amount int, novelId string, desc string) (*UserCredit, error)
```

#### 3.4.3 ChaincodeService
```go
type ChaincodeService struct{}

func (s *ChaincodeService) SaveNovel(novel *Novel) error
func (s *ChaincodeService) GetNovel(id string) (*Novel, error)
func (s *ChaincodeService) GetAllNovels() ([]*Novel, error)
func (s *ChaincodeService) DeleteNovel(id string) error
func (s *ChaincodeService) SaveUserCredit(uc *UserCredit) error
func (s *ChaincodeService) GetUserCredit(userId string) (*UserCredit, error)
func (s *ChaincodeService) GetAllUserCredits() ([]*UserCredit, error)
func (s *ChaincodeService) DeleteUserCredit(userId string) error
func (s *ChaincodeService) SaveCreditHistory(h *CreditHistory) error
func (s *ChaincodeService) GetCreditHistoriesByUser(userId string) ([]*CreditHistory, error)
```

#### 3.4.4 HMACService
```go
func GetRechargeSecretKey() string
func ComputeHMACSignature(params map[string]string, secretKey string) string
func ValidateHMACSignature(params map[string]string, receivedSignature string, secretKey string) bool
func ValidateTimestamp(timestamp int64) error
```

### 3.5 目录结构
```
token_blockchain/
├── main.go
├── go.mod
├── go.sum
├── .env
├── .gitignore
├── blockchain/
│   └── blockchain.go           # 区块链核心: DataType + Block + CRUD
├── database/
│   ├── models.go               # Novel, UserCredit, CreditHistory, RechargeRecord, User
│   └── mongodb.go              # MongoDB CRUD 操作
├── service/
│   ├── chaincode_service.go    # 区块链读写封装
│   ├── hmac_service.go         # HMAC 签名验证
│   ├── novel_service.go        # Novel 业务逻辑
│   └── user_service.go         # UserCredit + Token 业务逻辑
├── api/
│   └── server.go               # API 路由和 Handlers
└── docs/
    ├── consensus_mechanisms.md
    └── implementation-plan.md
```

---

## 4. 实施状态

### ✅ 已完成

| 任务 | 文件 | 状态 |
|------|------|------|
| T1.1: 改造 blockchain.go | `blockchain/blockchain.go` | ✅ 完成 |
| T1.2: 创建 models.go | `database/models.go` | ✅ 完成 |
| T1.3: 创建 mongodb.go | `database/mongodb.go` | ✅ 完成 |
| T2.1: 创建 chaincode_service.go | `service/chaincode_service.go` | ✅ 完成 |
| T2.2: 创建 novel_service.go | `service/novel_service.go` | ✅ 完成 |
| T2.3: 创建 user_service.go | `service/user_service.go` | ✅ 完成 |
| T3.1: 创建 api/server.go | `api/server.go` | ✅ 完成 |
| T4.1: 重写 main.go | `main.go` | ✅ 完成 |
| T4.2: 更新 go.mod | `go.mod` | ✅ 完成 |
| T4.3: 环境配置 .env | `.env` | ✅ 完成 |
| 补充: RechargeRecord 模型 | `database/models.go` | ✅ 完成 |
| 补充: RechargeRecord MongoDB 操作 | `database/mongodb.go` | ✅ 完成 |
| 补充: User 模型 | `database/models.go` | ✅ 完成 |
| 补充: User MongoDB 操作 | `database/mongodb.go` | ✅ 完成 |

---

## 5. 环境变量

```
ADDR=8080
MONGODB_URI=mongodb://admin:715705%40Qc123@127.0.0.1:27017
MONGODB_DATABASE=novel
RECHARGE_SECRET_KEY="HELLOWORxiaobai123@_715qc"
```

---

## 6. 验收测试

### 6.1 集成测试
- [ ] API: 创建小说 → 获取小说 → 更新小说 → 删除小说
- [ ] API: 创建用户 → 充值 → 消费 → 余额验证
- [ ] API: 余额不足时消费返回 400

### 6.2 数据一致性
- [ ] 区块链写入后 MongoDB 同步成功
- [ ] MongoDB 未命中时能从区块链回填
- [ ] CreditHistory 记录完整

---

## 7. 风险与限制

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 单节点无共识 | 数据一致性依赖单点 | MVP 阶段可接受 |
| 无持久化 | 服务重启数据丢失 | 后续添加文件存储 |
| 无分页 | 数据量大时性能问题 | 后续添加分页支持 |
| 无交易签名 | 无法验证操作者身份 | 后续添加 RSA 签名 |

---

## 8. 故意省略的原始功能

以下功能因架构差异未实现：

| 功能 | 原项目实现 | 省略原因 |
|------|-----------|----------|
| EventService | Fabric 事件监听 + MongoDB 同步 | 轻量区块链无事件机制 |
| RSA 加密中间件 | 请求/响应加密 | MVP 阶段不需要 |
| Debounce 中间件 | 500ms 防抖限流 | MVP 阶段不需要 |
| SSE 事件推送 | 实时推送新区块 | 后续可添加 |

---

## 9. 后续优化方向

1. **持久化**: 添加文件或 LevelDB 存储区块链数据
2. **分页**: GetAll 接口添加分页参数
3. **签名验证**: API 请求添加 RSA 签名验证
4. **事件推送**: SSE 实时推送新区块
5. **多节点**: 实现简单 Raft 共识
