# 架构设计

## 系统概述

AI Gateway 采用适配器模式设计，作为客户端与多个 AI 服务商之间的统一网关层。

## 架构图

```
                         ┌─────────────────────────────────────────────┐
                         │              AI Gateway                     │
                         │                                             │
┌──────────┐             │  ┌─────────┐    ┌─────────────────────┐    │
│          │   HTTP      │  │         │    │     Adapters        │    │
│  Client  │ ──────────► │  │ Router  │───►│                     │    │
│          │             │  │         │    │  ┌───────────────┐  │    │
└──────────┘             │  └─────────┘    │  │ GPT Adapter   │──┼────┼──► OpenAI API
                         │                 │  └───────────────┘  │    │
                         │                 │  ┌───────────────┐  │    │
                         │                 │  │Claude Adapter │──┼────┼──► Anthropic API
                         │                 │  └───────────────┘  │    │
                         │                 │  ┌───────────────┐  │    │
                         │                 │  │Gemini Adapter │──┼────┼──► Google API
                         │                 │  └───────────────┘  │    │
                         │                 └─────────────────────┘    │
                         └─────────────────────────────────────────────┘
```

## 核心组件

### 1. Router (路由器)

负责解析入站请求并路由到对应的适配器。

**职责:**
- 解析请求路径，识别目标服务商
- 请求校验和鉴权
- 负载均衡和故障转移

### 2. Adapters (适配器)

每个服务商对应一个适配器，负责请求/响应的格式转换。

#### GPT Adapter
- 目标: OpenAI API
- 端点: `https://api.openai.com/v1/`
- 支持: Chat Completions, Embeddings, Images

#### Anthropic Adapter
- 目标: Anthropic API
- 端点: `https://api.anthropic.com/v1/`
- 支持: Messages API

#### Gemini Adapter
- 目标: Google Gemini API
- 端点: `https://generativelanguage.googleapis.com/v1/`
- 支持: Generate Content

### 3. Request/Response Transformer

统一的请求响应转换层。

**请求转换流程:**
```
Client Request → 标准化格式 → 目标 API 格式 → 发送请求
```

**响应转换流程:**
```
API Response → 解析响应 → 标准化格式 → 返回客户端
```

## 数据流

1. 客户端发送请求到 Gateway
2. Router 解析路径，确定目标适配器
3. 适配器转换请求格式
4. 转发请求到目标 AI 服务
5. 接收响应并转换为标准格式
6. 返回标准化响应给客户端

## 统一消息格式

### 请求格式
```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "stream": false,
  "temperature": 0.7,
  "max_tokens": 1000
}
```

### 响应格式
```json
{
  "id": "msg_xxx",
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 10,
    "total_tokens": 30
  }
}
```

## 扩展性设计

- **新服务商接入**: 实现 `BaseAdapter` 接口即可添加新的 AI 服务商
- **中间件支持**: 支持添加请求/响应中间件进行日志、监控、限流等
- **配置化**: 通过配置文件管理 API 密钥、端点、模型映射等
