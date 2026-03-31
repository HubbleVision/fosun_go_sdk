# FosunXCZ OpenAPI Go SDK

复星XCZ OpenAPI 的 Go 语言 SDK，提供完整的会话管理、ECDH 密钥交换、请求签名和 AES-256-GCM 加解密功能。

## 目录结构

```
fosun_go_sdk/
├── auth/                  # 会话管理，实现 ECDH 握手和密钥派生
│   └── auth.go
├── client/                # API 客户端，封装请求签名和自动加解密
│   └── client.go
├── config/                # 配置加载，支持环境变量和 YAML 文件
│   └── config.go
├── crypto/                # 密码学工具，包含 ECDSA, ECDH, HKDF, AES-GCM 等
│   └── crypto.go
├── cmd/                   # 示例代码
│   └── main.go
├── go.mod                 # Go 模块依赖
└── README.md              # 本文档
```

## 功能特性

- **自动会话管理** - ECDH 握手建立会话，自动处理会话续期
- **请求签名** - HMAC-SHA256 签名，符合复星 API 规范
- **自动加解密** - AES-256-GCM 加密请求/解密响应
- **自动重试** - 网络波动时自动重试（默认3次）
- **并发安全** - 支持高并发调用
- **多配置方式** - 支持环境变量和 YAML 配置文件

## 安装

```bash
# 克隆项目
git clone https://github.com/your-repo/fosun_go_sdk.git
cd fosun_go_sdk

# 下载依赖
go mod tidy

# 运行示例
go run ./cmd/main.go
```

## 配置方式

### 方式一：环境变量

```bash
# 基础配置
export FOSUN_BASE_URL="https://openapi-uat.fosunxcz.com"
export FOSUN_API_KEY="ak_xxxxxxxxxxxxxxxxxxxxxx="

# 密钥配置（PEM 格式）
export FSOPENAPI_CLIENT_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC...
-----END PRIVATE KEY-----"

export FSOPENAPI_SERVER_PUBLIC_KEY="-----BEGIN PUBLIC KEY-----
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE...
-----END PUBLIC KEY-----"

# 可选配置
export SDK_TYPE="ops"           # SDK类型: "" 或 "ops"
export FOSUN_REQUEST_TIMEOUT=15 # 请求超时(秒)
export FOSUN_MAX_RETRIES=3     # 最大重试次数
```

### 方式二：YAML 配置文件

创建 `config.yaml` 文件：

```yaml
# config.yaml
baseURL: "https://openapi-uat.fosunxcz.com"
apiKey: "ak_xxxxxxxxxxxxxxxxxxxxxx="
clientPrivateKey: |
  -----BEGIN PRIVATE KEY-----
  MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC...
  -----END PRIVATE KEY-----
serverPublicKey: |
  -----BEGIN PUBLIC KEY-----
  MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE...
  -----END PUBLIC KEY-----
sdkType: ""
requestTimeout: 15
maxRetries: 3
```

加载配置：

```go
cfg, err := config.LoadConfig("config.yaml")
```

## 快速开始

```go
package main

import (
    "log"

    "hubble_fosun_sdk/client"
    "hubble_fosun_sdk/config"
)

func main() {
    // 加载配置（支持环境变量和YAML）
    cfg := config.LoadConfigFromEnv()

    // 创建客户端
    apiClient, err := client.NewFromConfig(client.Config{
        BaseURL:          cfg.BaseURL,
        APIKey:           cfg.APIKey,
        ClientPrivateKey: cfg.ClientPrivateKey,
        ServerPublicKey:  cfg.ServerPublicKey,
    })
    if err != nil {
        log.Fatalf("初始化失败: %v", err)
    }

    // 查询港股K线
    resp, err := apiClient.QueryKlineByMarket("hk", "00700", "day", 10, 0, 0)
    if err != nil {
        log.Fatalf("查询失败: %v", err)
    }
    log.Printf("港股K线: %+v", resp)
}
```

## API 接口

### 客户端方法

| 方法 | 描述 |
|------|------|
| `NewOpenAPIClient(baseURL, apiKey)` | 创建客户端（使用环境变量中的密钥） |
| `NewFromConfig(Config)` | 使用配置创建客户端 |
| `Get(path, params)` | 发送 GET 请求 |
| `Post(path, data)` | 发送 POST 请求（自动加密） |
| `Request(method, path, data, params)` | 通用请求方法 |
| `QueryKline(code, ktype, num, startTime, endTime)` | 查询K线（默认上海市场） |
| `QueryKlineByMarket(market, code, ktype, num, startTime, endTime)` | 按市场查询K线 |

### K线查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `market` | string | 市场: `sh`(上海), `sz`(深圳), `hk`(港股), `us`(美股) |
| `code` | string | 股票代码: `600036`, `00700`, `AAPL` |
| `ktype` | string | K线类型: `1min`, `5min`, `15min`, `30min`, `60min`, `day`, `week`, `month` |
| `num` | int | 返回条数，0表示全部 |
| `startTime` | int64 | 开始时间戳(毫秒) |
| `endTime` | int64 | 结束时间戳(毫秒) |

### 行情数据返回字段

| 字段 | 说明 |
|------|------|
| `time` | 时间 |
| `open` | 开盘价 |
| `high` | 最高价 |
| `low` | 最低价 |
| `close` | 收盘价 |
| `vol` | 成交量 |
| `turnover` | 成交额 |
| `pClose` | 前收盘价 |
| `tor` | 换手率 |

## 其他项目引用

在 `go.mod` 中添加 replace 指令：

```go
require (
    hubble_fosun_sdk v0.0.0
)

replace hubble_fosun_sdk => C:/Users/xiaomager/Downloads/fosun_go_sdk
```

然后运行：

```bash
go mod tidy
```

## 依赖

- `github.com/google/uuid` - 生成唯一请求ID
- `golang.org/x/crypto` - ECDH/HKDF/AES-GCM
- `gopkg.in/yaml.v3` - YAML 配置解析

## 注意事项

1. **密钥安全** - 长期密钥不要硬编码在代码中，使用环境变量或密钥管理服务
2. **会话有效期** - SDK 会自动处理会话续期（提前60秒）
3. **加密规则** - 行情接口(`/market/`)不加密，其他接口自动加密
4. **错误处理** - 业务错误会返回 error，HTTP 状态码非200也会返回 error