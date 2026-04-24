# VREC - 语音识别服务

语音识别（ASR）后端服务，支持音频上传、转写、字幕生成、会议纪要。

## 项目架构

```
┌─────────────────────────────────────────────────────────────┐
│                         Gin Router                          │
├──────────┬──────────┬──────────┬──────────┬──────────────┤
│  User    │  Order   │ Recharge │   Payment    │
│ Handler  │ Handler  │ Handler  │  Callback   │
└────┬─────┴────┬─────┴────┬─────┴────┬─────┴──────┬──────┘
     │          │          │          │            │
┌────▼──────────▼──────────▼──────────▼────────────▼──────┐
│                      Service Layer                        │
├──────────┬──────────┬──────────┬──────────┬──────────────┤
│  User    │  Order   │Recharge  │Transcrip-│  Payment     │
│ Service  │ Service  │ Service  │tionResult│ Service      │
│          │          │          │ Service  │              │
│          │          │          │ S3Service│              │
│          │          │          │Subtitle  │              │
│          │          │          │MeetingNote│             │
└────┬─────┴────┬─────┴────┬─────┴────┬─────┴──────┬──────┘
     │          │          │          │            │
┌────▼──────────▼──────────▼──────────▼────────────▼──────┐
│                    Repository Layer                      │
├──────────┬──────────┬──────────┬──────────┬──────────────┤
│  User    │  Order   │Recharge  │Transcrip-│  User        │
│ Repository│ Repository│OrderRepo│tionResult│TokenRepo     │
│          │          │          │Repo      │              │
│          │          │          │MeetingSum│              │
│          │          │          │Repo      │              │
└────┬─────┴────┬─────┴────┬─────┴────┬─────┴──────┬──────┘
     │          │          │          │            │
     ▼          ▼          ▼          ▼            ▼
┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
│PostgreSQL│ │PostgreSQL│ │PostgreSQL│ │PostgreSQL│ │ S3/MinIO│
│  (用户) │ │ (订单)  │ │ (充值) │ │ (结果) │ │ (音频) │
└─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘
```

## 技术栈

- **Web 框架**: Gin
- **数据库**: PostgreSQL + pgx/v5
- **对象存储**: AWS S3 SDK v2 (支持 MinIO 等兼容存储)
- **音频处理**: FFmpeg + FFprobe
- **日志**: Uber zap + lumberjack (日志切割)
- **配置**: TOML (pelletier/go-toml/v2)
- **ASR**: 阿里云 FunAudio ASR (REST API, 轮询模式)
- **LLM**: 支持会议纪要生成

## S3 存储策略

音频文件存储在单个 S3 bucket 中，key 包含完整路径信息：

- **Bucket 命名**: 固定 bucket 名称（如 `vrec`）
- **Key 格式**: `{bucket}/{prefix}/{YYMMDD}/{filename}-{uuid}.{ext}`（如 `vrec/audio/260425/recording-550e8400-e29b-41d4-a716-446655440000.wav`）
  - 远程 URL 场景从 URL 中提取文件名
  - 系统生成文件（如字幕、转写结果）使用固定文件名（如 `vrec/subtitle/260425/subtitle-uuid.srt`）
- **文件名清理**: 原始文件名中的非法字符（`/ \ : * ? " < > | 空格`）会被替换为下划线
- **数据清理**: 可按 key 前缀清理（如删除 `vrec/audio/2604*` 删除 2026 年 4 月的音频文件）

## 目录结构

```
vrec/
├── cmd/
│   └── main.go              # 应用入口
├── internal/
│   ├── config/              # 配置加载
│   ├── handler/             # HTTP 请求处理
│   │   ├── user.go         # 用户注册/登录/登出
│   │   ├── order.go        # 订单/转写/字幕/会议纪要
│   │   ├── recharge.go     # 充值订单
│   │   └── payment.go      # 支付回调 (支付宝/微信)
│   ├── middleware/          # 中间件 (认证/QPS限制/日志)
│   ├── model/              # 数据模型
│   ├── repository/         # 数据库访问
│   ├── service/            # 业务逻辑
│   │   ├── asr.go         # ASR 服务
│   │   ├── asr_worker.go  # ASR 轮询 worker
│   │   ├── s3.go          # S3 服务
│   │   ├── subtitle.go    # 字幕生成
│   │   └── meeting_note.go # 会议纪要生成
│   └── pkg/
│       ├── errors/         # 统一错误码
│       └── response/       # 统一响应格式
├── schema.sql              # 数据库表结构
├── config.toml.example     # 配置示例
└── README.md
```

## 配置说明

```toml
[server]
port = "8080"

[database]
host = "localhost"
port = "5432"
username = "postgres"
password = "postgres"
name = "vrec"

[s3]
endpoint = "s3.amazonaws.com"   # S3 端点 (支持 MinIO)
access_key = ""                # 访问密钥
secret_key = ""                # 密钥
bucket = "vrec"                # Bucket 前缀（实际 bucket 为 {bucket}-YYYY-MM）
region = "us-east-1"            # 区域
url_expire_days = 3            # 预签名 URL 有效期（天）

[asr]
api_key = ""                    # 阿里云 FunAudio ASR API Key

[llm]
enabled = false                 # 是否启用 LLM
api_key = ""                   # LLM API Key
model = ""                     # LLM 模型

[pricing]
storage_per_gb_day = 0.01      # 存储价格（元/GB/天）
asr_per_minute = 0.1           # ASR 转写价格（元/分钟）
subtitle_per_minute = 0.05     # 字幕价格（元/分钟）
meeting_note_per_token = 0.001 # 会议纪要价格（元/千token）
low_balance_threshold = 10     # 低余额阈值（元）
low_balance_qps = 1            # 低余额时 QPS 限制
local_storage_threshold = 1048576 # 小于此值（1MB）的结果存数据库

[logger]
level = "info"                  # 日志级别：debug/info/warn/error
format = "json"                # 日志格式：json/console
path = "app.log"               # 日志文件路径，为空则仅输出到 stderr
max_size = 100                 # 单个日志文件最大大小（MB）
max_backups = 30               # 保留的旧日志文件数量
max_age = 7                    # 旧日志文件保留天数（天）
compress = true                # 是否压缩旧日志文件

[auth]
token_expire_days = 7           # Token 有效期（天）
jwt_enabled = false             # 是否启用 JWT 模式
jwt_secret = "your-secret-key-change-in-production"  # JWT 签名密钥
sid_secret = "your-sid-secret-change-in-production" # SID 生成密钥
```

## 认证模式

系统支持两种认证模式，通过配置 `auth.jwt_enabled` 切换：

### UUID Token 模式（默认）
- 适用于 API 调用场景
- 用户登录后返回 UUID 格式 token
- 每次登录生成新 Token，支持多设备登录
- Token 存储在数据库，可管理和撤销

### JWT 模式
- 适用于 Web UI 场景
- 用户登录后返回 JWT 格式 token
- 无状态认证，无需查询数据库
- 启用方式：设置 `auth.jwt_enabled = true`
- JWT 模式下不支持多设备 token 管理和撤销

## SID 请求追踪

每个 HTTP 请求都会生成唯一的 SID（Session ID），用于日志追踪和问题排查：

- **Header**: 请求时可传入 `X-Sid`，不传则自动生成
- **响应**: 响应的 `X-Sid` header 和响应 body 的 `sid` 字段返回同一个 SID
- **日志**: 每个请求的日志都会记录对应的 SID
- **格式**: `base64(signature|ip|timestamp|seq)`，包含 HMAC 签名防篡改
- **解析**: 可从 SID 中提取服务器 IP、时间戳和序列号

## API 接口文档

### 认证接口（公开）

#### POST /register - 用户注册

**请求**
```json
{
  "username": "string",  // 3-32字符
  "password": "string"   // 6-32字符
}
```

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": 1,
    "username": "testuser",
    "balance": "0.00"
  }
}
```

**响应错误**
| code | msg                 |
| ---- | ------------------- |
| 1002 | invalid parameters  |
| 3002 | user already exists |

---

#### POST /login - 用户登录

**请求**
```json
{
  "username": "string",
  "password": "string"
}
```

**响应成功 (200)** - UUID Token 模式
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": 1,
    "username": "testuser",
    "balance": "100.00",
    "token": "550e8400-e29b-41d4-a716-446655440000",
    "expires_at": "2026-05-01T00:00:00Z",
    "token_type": "uuid"
  }
}
```

**响应成功 (200)** - JWT 模式
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": 1,
    "username": "testuser",
    "balance": "100.00",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "jwt"
  }
}
```

**响应错误**
| code | msg                          |
| ---- | ---------------------------- |
| 1002 | invalid parameters           |
| 2004 | invalid username or password |

---

### Token 管理接口（需认证）

#### GET /tokens - 查询 Token 列表

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "tokens": [
      {
        "id": 1,
        "token": "550e8400-e29b-41d4-a716-446655440000",
        "created_at": "2026-04-24T10:00:00Z",
        "expires_at": "2026-05-01T00:00:00Z"
      }
    ]
  }
}
```

---

#### DELETE /tokens/:token_id - 删除指定 Token

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": null
}
```

**响应错误**
| code | msg                |
| ---- | ------------------ |
| 1002 | invalid parameters |
| 2002 | invalid token      |

---

### 订单接口（需认证）

认证方式：`Authorization: Bearer <token>`

#### POST /orders - 远程 URL 创建订单

**请求**
```json
{
  "audio_url": "https://example.com/audio.mp3",
  "need_subtitle": false,       // 是否需要字幕（可选，默认false）
  "need_meeting_note": false,  // 是否需要会议纪要（可选，默认false）
  "callback_url": ""            // 回调地址（可选，任务完成时推送结果）
}
```

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "status": "processing",
    "audio_duration": 120,
    "storage_cost": "0.00003",
    "asr_cost": "0.20",
    "subtitle_cost": "0.00",
    "total_cost": "0.20003"
  }
}
```

**响应错误**
| code | msg                                  |
| ---- | ------------------------------------ |
| 1002 | invalid parameters                   |
| 1003 | audio file exceeds 1GB limit         |
| 1004 | audio duration exceeds 6 hours limit |
| 4001 | insufficient balance                 |

---

#### POST /orders/stream - 流式上传创建订单

**请求** (multipart/form-data)
```
audio: <file>
need_subtitle: false
need_meeting_note: false
callback_url: ""
```

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "status": "processing",
    "audio_duration": 120,
    "storage_cost": "0.00003",
    "asr_cost": "0.20",
    "subtitle_cost": "0.00",
    "total_cost": "0.20003"
  }
}
```

---

#### GET /orders/:order_no - 查询订单

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "status": "success",
    "original_url": "https://example.com/audio.mp3",
    "audio_duration": 120,
    "audio_format": "mp3",
    "sample_rate": 44100,
    "channels": 2,
    "bit_rate": 128000,
    "codec": "mp3",
    "s3_url": "https://s3.../audio/2026/04/24/xxx.wav",
    "s3_expired": false,
    "storage_cost": "0.00003",
    "asr_cost": "0.20",
    "subtitle_cost": "0.00",
    "total_cost": "0.20003",
    "need_subtitle": false,
    "need_meeting_note": false,
    "created_at": "2026-04-24T10:00:00Z"
  }
}
```

**响应错误**
| code | msg                |
| ---- | ------------------ |
| 1002 | invalid parameters |
| 5001 | order not found    |

---

#### GET /orders - 订单列表

**查询参数**
| 参数           | 类型   | 说明                                            |
| -------------- | ------ | ----------------------------------------------- |
| limit          | string | 1-100，默认20                                   |
| after_order_no | string | 上一页最后一条的 order_no，用于分页             |
| after_time     | string | RFC3339 格式，如 2026-01-01T00:00:00Z，用于分页 |

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "orders": [
      {
        "order_no": "550e8400-e29b-41d4-a716-446655440000",
        "status": "success",
        "audio_duration": 120,
        "total_cost": "0.20003",
        "created_at": "2026-04-24T10:00:00Z"
      }
    ]
  }
}
```

---

#### GET /results/:order_no - 转写结果

返回与 ASR 格式一致的结构化数据。

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "file_url": "https://...",
    "properties": {
      "audio_format": "wav",
      "channels": [0, 1],
      "original_sampling_rate": 16000,
      "original_duration_in_milliseconds": 120000
    },
    "transcripts": [
      {
        "channel_id": 0,
        "content_duration_in_milliseconds": 5000,
        "text": "原始文本",
        "sentences": [
          {
            "begin_time": 0,
            "end_time": 5000,
            "text": "句子文本",
            "sentence_id": 1,
            "speaker_id": 1,
            "words": [...],
            "punctuation": "。"
          }
        ]
      }
    ]
  }
}
```

**响应错误**
| code | msg                |
| ---- | ------------------ |
| 1002 | invalid parameters |
| 5003 | result not found   |

---

#### DELETE /orders/:order_no - 取消订单

取消处理中的订单，ASR 任务将不再查询。

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": null
}
```

---

#### GET /orders/insufficient - 查询余额不足的订单

**查询参数**
| 参数           | 类型   | 说明                                            |
| -------------- | ------ | ----------------------------------------------- |
| limit          | string | 1-100，默认20                                   |
| after_order_no | string | 上一页最后一条的 order_no，用于分页             |
| after_time     | string | RFC3339 格式，如 2026-01-01T00:00:00Z，用于分页 |

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "orders": [
      {
        "order_no": "550e8400-e29b-41d4-a716-446655440000",
        "status": "insufficient",
        "total_cost": "0.20003",
        "created_at": "2026-04-24T10:00:00Z"
      }
    ]
  }
}
```

---

#### POST /orders/retry - 批量重试余额不足的订单

**请求**
```json
{
  "order_nos": [
    "550e8400-e29b-41d4-a716-446655440000",
    "660e8400-e29b-41d4-a716-446655440001"
  ]
}
```

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "success": ["550e8400-e29b-41d4-a716-446655440000"],
    "failed": [
      {
        "order_no": "660e8400-e29b-41d4-a716-446655440001",
        "error": "insufficient balance"
      }
    ]
  }
}
```

---

#### GET /orders/:order_no/cost - 查询订单费用

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "storage_cost": "0.00003",
    "asr_cost": "0.20",
    "subtitle_cost": "0.00",
    "meeting_cost": "0.00",
    "total_cost": "0.20003"
  }
}
```

---

#### GET /bills - 查询用户账单

**查询参数**
| 参数       | 类型   | 说明                                  |
| ---------- | ------ | ------------------------------------- |
| start_time | string | RFC3339 格式，如 2026-01-01T00:00:00Z |
| end_time   | string | RFC3339 格式，如 2026-12-31T23:59:59Z |
| page       | string | 页码，默认1                           |
| page_size  | string | 每页数量，默认20，最大100             |

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "start_time": "2026-01-01T00:00:00Z",
    "end_time": "2026-12-31T23:59:59Z",
    "total": 12,
    "page": 1,
    "page_size": 20,
    "bills": [
      {
        "month": "2026-01",
        "orders": [
          {
            "order_no": "550e8400-e29b-41d4-a716-446655440000",
            "storage_cost": "0.00003",
            "asr_cost": "0.20",
            "subtitle_cost": "0.00",
            "meeting_cost": "0.00",
            "total_cost": "0.20003",
            "created_at": "2026-01-15T10:00:00Z"
          }
        ],
        "recharges": [
          {
            "recharge_no": "660e8400-e29b-41d4-a716-446655440001",
            "amount": "100.00",
            "created_at": "2026-01-10T10:00:00Z"
          }
        ],
        "total_cost": "0.20003",
        "total_recharge": "100.00",
        "order_count": 1,
        "recharge_count": 1
      }
    ]
  }
}
```

---

### 字幕接口（需认证）

#### POST /subtitles/:order_no - 生成字幕

订单状态为成功后可调用。

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "subtitle_content": "1\n00:00:00,000 --> 00:00:05,000\n字幕文本\n\n2\n00:00:05,000 --> 00:00:10,000\n第二段字幕",
    "cost": "0.10"
  }
}
```

**响应错误**
| code | msg                 |
| ---- | ------------------- |
| 5002 | order not succeeded |

---

#### GET /subtitles/:order_no - 获取字幕

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "subtitle_content": "1\n00:00:00,000 --> 00:00:05,000\n字幕文本..."
  }
}
```

---

### 会议纪要接口（需认证）

#### POST /meeting_notes/:order_no - 生成会议纪要

订单状态为成功后可调用，需要启用 LLM。

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "summary_text": "# 会议纪要\n\n## 会议主题\n...\n\n## 讨论内容\n...",
    "cost": "0.50"
  }
}
```

**响应错误**
| code | msg                 |
| ---- | ------------------- |
| 5002 | order not succeeded |
| 7001 | llm not enabled     |

---

#### GET /meeting_notes/:order_no - 获取会议纪要

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "order_no": "550e8400-e29b-41d4-a716-446655440000",
    "summary_text": "# 会议纪要\n\n## 会议主题\n..."
  }
}
```

---

### 充值接口（需认证）

#### POST /recharge - 创建充值订单

**请求**
```json
{
  "amount": "100.00",
  "pay_channel": "alipay"  // alipay | wechat
}
```

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "recharge_no": "550e8400-e29b-41d4-a716-446655440000",
    "pay_url": "https://pay.example.com/xxx",
    "amount": "100.00",
    "pay_channel": "alipay",
    "expires_at": "2026-04-25T10:00:00Z"
  }
}
```

---

#### GET /recharge/:recharge_no - 查询充值订单

**响应成功 (200)**
```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "recharge_no": "550e8400-e29b-41d4-a716-446655440000",
    "status": "paid",
    "amount": "100.00",
    "pay_channel": "alipay",
    "pay_url": "https://pay.example.com/xxx",
    "paid_at": "2026-04-24T10:05:00Z",
    "expires_at": "2026-04-25T10:00:00Z",
    "created_at": "2026-04-24T10:00:00Z"
  }
}
```

---

#### GET /recharges - 充值订单列表

**查询参数**
| 参数           | 类型   | 说明                                            |
| -------------- | ------ | ----------------------------------------------- |
| limit          | string | 1-100，默认20                                   |
| after_order_no | string | 上一页最后一条的 recharge_no，用于分页          |
| after_time     | string | RFC3339 格式，如 2026-01-01T00:00:00Z，用于分页 |

---

### 回调接口（公开）

#### POST /callback/alipay - 支付宝回调

---

#### POST /callback/wechat - 微信支付回调

---

## 业务流程

### 1. 音频转写流程

```
1. 用户上传音频 (流式/远程URL)
         │
         ▼
2. 服务端解析音频元数据 (FFprobe)，校验大小和时长限制
         │
         ▼
3. 上传音频到 S3，生成预签名 URL
         │
         ▼
4. 扣减用户余额 (存储费 + ASR费 + 可选字幕费)
         │
         ▼
5. 提交 ASR 任务到阿里云 FunAudio
         │
         ▼
6. 后台 Worker 每分钟轮询任务状态
         │
         ▼
7. 任务完成后下载结果、上传到 S3、保存到数据库
         │
         ▼
8. 如果有回调地址，推送结构化结果
```

### 2. 订单取消流程

```
1. 用户发起取消请求 DELETE /orders/:order_no
         │
         ▼
2. 验证订单状态为 processing
         │
         ▼
3. 通知 ASR 取消任务（如果 task_id 存在）
         │
         ▼
4. 订单状态更新为 canceled
```

### 3. 充值流程

```
1. 用户发起充值请求
         │
         ▼
2. 创建充值订单，生成支付链接
         │
         ▼
3. 用户跳转支付宝/微信支付
         │
         ▼
4. 支付完成后第三方回调
         │
         ▼
5. 增加用户余额
```

## 音频限制

| 限制项   | 最大值 |
| -------- | ------ |
| 文件大小 | 1GB    |
| 时长     | 6小时  |

## 费用计算

```
订单总费用 = 存储费用 + ASR 费用 + 字幕费用（可选）

存储费用 = 文件大小(GB) × 存储天数 × 存储单价
ASR 费用 = 音频时长(分钟) × ASR 单价
字幕费用 = 音频时长(分钟) × 字幕单价（可选）
会议纪要费用 = Token数 / 1000 × 会议纪要单价（生成时计算）
```

例：1分钟音频（约 1MB）

- 存储费用：1MB / 1024 × 3天 × 0.01元 = 0.00003元
- ASR 费用：1分钟 × 0.1元 = 0.1元
- 总费用：约 0.1元

## 错误码

| 区间 | 类别     |
| ---- | -------- |
| 1xxx | 通用错误 |
| 2xxx | 认证错误 |
| 3xxx | 用户错误 |
| 4xxx | 余额错误 |
| 5xxx | 订单/业务错误 |
| 6xxx | 充值错误 |

### 通用错误 (1xxx)

| code | msg                                  | 说明              |
| ---- | ------------------------------------ | ----------------- |
| 1001 | internal error                       | 服务器内部错误    |
| 1002 | invalid parameters                   | 参数错误          |
| 1003 | audio file exceeds 1GB limit         | 音频文件超过1GB   |
| 1004 | audio duration exceeds 6 hours limit | 音频时长超过6小时 |
| 1006 | rate limit exceeded                  | 超出QPS限制       |

### 认证错误 (2xxx)

| code | msg                          | 说明             |
| ---- | ---------------------------- | ---------------- |
| 2001 | missing authorization header | 缺少认证头       |
| 2002 | invalid token                | 无效token        |
| 2003 | token expired                | token已过期      |
| 2004 | invalid username or password | 用户名或密码错误 |

### 用户错误 (3xxx)

| code | msg                 | 说明       |
| ---- | ------------------- | ---------- |
| 3001 | user not found      | 用户不存在 |
| 3002 | user already exists | 用户已存在 |
| 3003 | invalid password    | 密码错误   |

### 余额错误 (4xxx)

| code | msg                  | 说明     |
| ---- | -------------------- | -------- |
| 4001 | insufficient balance | 余额不足 |

### 订单/业务错误 (5xxx)

| code | msg                    | 说明           |
| ---- | ---------------------- | -------------- |
| 5001 | order not found        | 订单不存在     |
| 5002 | invalid order status   | 无效订单状态   |
| 5003 | result not found       | 结果不存在     |
| 5004 | subtitle not found     | 字幕不存在     |
| 5005 | meeting note not found | 会议纪要不存在 |
| 5006 | llm not enabled        | LLM 未启用     |

### 充值错误 (6xxx)

| code | msg                      | 说明           |
| ---- | ------------------------ | -------------- |
| 6001 | recharge order not found | 充值订单不存在 |
| 6002 | recharge order expired   | 充值订单已过期 |
| 6003 | recharge already paid    | 充值已支付     |
| 6004 | invalid pay channel      | 无效支付渠道   |

## 数据库表

### orders - 音频订单表

| 字段              | 类型      | 说明                                                            |
| ----------------- | --------- | --------------------------------------------------------------- |
| id                | BIGSERIAL | 自增 ID                                                         |
| order_no          | UUID      | 订单号                                                          |
| user_id           | BIGINT    | 用户 ID                                                         |
| token_id          | BIGINT    | Token ID (多设备)                                               |
| status            | VARCHAR   | pending/processing/success/failed/canceled/expired/insufficient |
| task_id           | VARCHAR   | ASR 任务 ID                                                     |
| callback_url      | TEXT      | 回调地址                                                        |
| original_url      | TEXT      | 原始音频地址                                                    |
| source            | VARCHAR   | local/remote/stream                                             |
| audio_duration    | BIGINT    | 音频时长（秒）                                                  |
| audio_format      | VARCHAR   | 音频格式                                                        |
| sample_rate       | BIGINT    | 采样率                                                          |
| channels          | INT       | 声道数                                                          |
| bit_rate          | BIGINT    | 比特率                                                          |
| codec             | VARCHAR   | 编解码器                                                        |
| s3_key            | TEXT      | S3 对象 key                                                     |
| s3_url            | TEXT      | S3 预签名 URL                                                   |
| s3_expires_at     | TIMESTAMP | S3 URL 过期时间                                                 |
| storage_cost      | DECIMAL   | 存储费用                                                        |
| asr_cost          | DECIMAL   | ASR 费用                                                        |
| subtitle_cost     | DECIMAL   | 字幕费用                                                        |
| meeting_cost      | DECIMAL   | 会议纪要费用                                                    |
| total_cost        | DECIMAL   | 总费用                                                          |
| need_subtitle     | BOOLEAN   | 是否需要字幕                                                    |
| need_meeting_note | BOOLEAN   | 是否需要会议纪要                                                |
| created_at        | TIMESTAMP | 创建时间                                                        |
| updated_at        | TIMESTAMP | 更新时间                                                        |

### transcription_results - 转写结果表

| 字段            | 类型      | 说明                     |
| --------------- | --------- | ------------------------ |
| id              | BIGSERIAL | 自增 ID                  |
| order_no        | UUID      | 订单号                   |
| result_s3_key   | TEXT      | S3 存储 key              |
| result_text     | TEXT      | 转写文本（小于阈值时存） |
| subtitle_s3_key | TEXT      | 字幕 S3 key              |
| created_at      | TIMESTAMP | 创建时间                 |

### meeting_summaries - 会议纪要表

| 字段           | 类型      | 说明         |
| -------------- | --------- | ------------ |
| id             | BIGSERIAL | 自增 ID      |
| order_no       | UUID      | 订单号       |
| summary_s3_key | TEXT      | S3 存储 key  |
| summary_text   | TEXT      | 会议纪要文本 |
| cost           | DECIMAL   | 生成费用     |
| created_at     | TIMESTAMP | 创建时间     |

### users - 用户表

| 字段       | 类型      | 说明           |
| ---------- | --------- | -------------- |
| id         | BIGSERIAL | 自增 ID        |
| username   | VARCHAR   | 用户名         |
| password   | VARCHAR   | 密码（bcrypt） |
| balance    | DECIMAL   | 余额           |
| qps_limit  | INT       | QPS 限制       |
| created_at | TIMESTAMP | 创建时间       |
| updated_at | TIMESTAMP | 更新时间       |

### user_tokens - 用户Token表

| 字段       | 类型      | 说明     |
| ---------- | --------- | -------- |
| id         | BIGSERIAL | 自增 ID  |
| user_id    | BIGINT    | 用户 ID  |
| token      | UUID      | Token    |
| created_at | TIMESTAMP | 创建时间 |
| expires_at | TIMESTAMP | 过期时间 |

### user_recharges - 用户充值记录表

| 字段       | 类型      | 说明     |
| ---------- | --------- | -------- |
| id         | BIGSERIAL | 自增 ID  |
| user_id    | BIGINT    | 用户 ID  |
| token_id   | BIGINT    | Token ID |
| amount     | DECIMAL   | 充值金额 |
| created_at | TIMESTAMP | 创建时间 |

### recharge_orders - 充值订单表

| 字段        | 类型      | 说明                 |
| ----------- | --------- | -------------------- |
| id          | BIGSERIAL | 自增 ID              |
| recharge_no | UUID      | 充值订单号           |
| user_id     | BIGINT    | 用户 ID              |
| token_id    | BIGINT    | Token ID             |
| amount      | DECIMAL   | 充值金额             |
| pay_channel | VARCHAR   | alipay/wechat        |
| status      | VARCHAR   | pending/paid/expired |
| trade_no    | VARCHAR   | 第三方交易号         |
| pay_url     | TEXT      | 支付链接             |
| expires_at  | TIMESTAMP | 过期时间             |
| paid_at     | TIMESTAMP | 支付时间             |
| created_at  | TIMESTAMP | 创建时间             |
| updated_at  | TIMESTAMP | 更新时间             |
