# API 参考

## 概述

AI Gateway 提供三个主要接口端点，分别对应三种 AI 服务商。所有接口均采用 REST 风格，支持 JSON 格式。

## 通用说明

### 基础 URL
```
http://localhost:8080/v1
```

### 认证方式
所有请求需在 Header 中携带 API Key:
```
Authorization: Bearer <your-api-key>
```

### 通用响应格式

**成功响应:**
```json
{
  "id": "msg_xxx",
  "model": "model-name",
  "choices": [...],
  "usage": {...}
}
```

**错误响应:**
```json
{
  "error": {
    "code": "error_code",
    "message": "Error description"
  }
}
```

---

## GPT 接口

### Chat Completions

创建对话补全请求。

**端点:** `POST /v1/gpt/chat/completions`

**请求参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称，如 `gpt-4`, `gpt-4o`, `gpt-3.5-turbo` |
| messages | array | 是 | 对话消息数组 |
| stream | boolean | 否 | 是否流式返回，默认 `false` |
| temperature | number | 否 | 采样温度 0-2，默认 `1` |
| max_tokens | integer | 否 | 最大生成 token 数 |

**请求示例:**
```bash
curl -X POST http://localhost:8080/v1/gpt/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

**响应示例:**
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21
  }
}
```

---

## Anthropic 接口

### Messages

创建 Claude 消息请求。

**端点:** `POST /v1/anthropic/messages`

**请求参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称，如 `claude-3-5-sonnet-20241022`, `claude-3-opus-20240229` |
| messages | array | 是 | 对话消息数组 |
| system | string | 否 | 系统提示词 |
| stream | boolean | 否 | 是否流式返回，默认 `false` |
| max_tokens | integer | 是 | 最大生成 token 数 |
| temperature | number | 否 | 采样温度 0-1，默认 `1` |

**请求示例:**
```bash
curl -X POST http://localhost:8080/v1/anthropic/messages \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

**响应示例:**
```json
{
  "id": "msg_xxx",
  "type": "message",
  "model": "claude-3-5-sonnet-20241022",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help you today?"
    }
  ],
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 15
  }
}
```

---

## Gemini 接口

### Generate Content

创建 Gemini 内容生成请求。

**端点:** `POST /v1/gemini/generateContent`

**请求参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称，如 `gemini-pro`, `gemini-pro-vision` |
| contents | array | 是 | 内容数组 |
| generationConfig | object | 否 | 生成配置 |
| safetySettings | array | 否 | 安全设置 |

**请求示例:**
```bash
curl -X POST http://localhost:8080/v1/gemini/generateContent \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-pro",
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "Hello!"}]
      }
    ]
  }'
```

**响应示例:**
```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {"text": "Hello! How can I help you today?"}
        ],
        "role": "model"
      },
      "finishReason": "STOP"
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 2,
    "candidatesTokenCount": 10,
    "totalTokenCount": 12
  }
}
```

---

## 流式响应

所有接口均支持流式响应 (Server-Sent Events)。设置 `stream: true` 启用。

**流式响应格式:**
```
data: {"id":"xxx","choices":[{"delta":{"content":"Hello"}}]}

data: {"id":"xxx","choices":[{"delta":{"content":"!"}}]}

data: [DONE]
```

---

## 错误码

| 错误码 | HTTP 状态码 | 说明 |
|--------|-------------|------|
| invalid_api_key | 401 | API Key 无效或缺失 |
| invalid_request | 400 | 请求参数错误 |
| model_not_found | 404 | 模型不存在 |
| rate_limit_exceeded | 429 | 请求频率超限 |
| internal_error | 500 | 服务器内部错误 |
| upstream_error | 502 | 上游 AI 服务错误 |
