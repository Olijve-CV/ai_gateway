# 快速开始

## 环境要求

- Go 1.21+
- 有效的 AI 服务商 API Key（OpenAI / Anthropic / Gemini 或自定义）

## 安装

```bash
# 克隆项目
git clone https://github.com/Olijve-CV/ai_gateway.git
cd ai_gateway

# 下载依赖
go mod download
```

## 配置

### 1. 复制环境变量模板

```bash
cp .env.example .env
```

- 可按需修改 `HOST`、`PORT`、`DATABASE_URL`。
- `JWT_SECRET` 与 `ENCRYPTION_KEY` 未设置时会自动生成（生产环境请显式配置）。

### 2. 启动服务

```bash
go run ./cmd/server
```

服务启动后访问: `http://localhost:8080`

### 3. 初始化服务商与网关 API Key

- 访问 `http://localhost:8080/register` 注册并登录。
- 在 Dashboard → Providers 添加服务商配置（API Key、Base URL、协议）。
- 在 Dashboard → API Keys 创建网关 API Key 供客户端调用。

## 验证安装

`API_KEY` 为 Dashboard 中创建的网关 API Key。

### OpenAI Chat Completions

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Say hello"}]
  }'
```

### Anthropic Messages

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Say hello"}]
  }'
```

### Gemini Generate Content

```bash
curl -X POST "http://localhost:8080/v1/models/gemini-pro:generateContent" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role": "user", "parts": [{"text": "Say hello"}]}]
  }'
```

## 下一步

- 阅读 [架构设计](architecture.md) 了解系统设计
- 查看 [API 参考](api-reference.md) 了解完整接口文档
