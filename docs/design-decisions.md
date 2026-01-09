# 设计决策

## 已确定的设计决策

### 1. 技术栈
- **语言/框架**: Python + FastAPI
- **HTTP 客户端**: httpx (支持异步和流式)
- **数据校验**: Pydantic v2

### 2. API Key 管理
- **模式**: 透传模式
- **实现**: 客户端在请求中携带目标服务商的 API Key，网关原样转发
- **Header**: `Authorization: Bearer <api-key>` 或 `x-api-key: <api-key>`

### 3. 格式转换
- **范围**: OpenAI / Anthropic / Gemini 三种格式互转
- **方向**: 任意格式 → 任意服务商

### 4. 错误处理
- **策略**: 透传上游错误
- **实现**: 将上游服务商返回的错误响应原样返回给客户端

### 5. 流式响应
- **支持**: 必须支持 SSE 流式输出
- **实现**: StreamingResponse + 实时格式转换

---

## 格式转换设计

### 转换矩阵

| 请求格式 | 目标服务 | 说明 |
|----------|----------|------|
| OpenAI 格式 | → OpenAI | 直接代理 |
| OpenAI 格式 | → Anthropic | 转换后调用 Claude |
| OpenAI 格式 | → Gemini | 转换后调用 Gemini |
| Anthropic 格式 | → OpenAI | 转换后调用 GPT |
| Anthropic 格式 | → Anthropic | 直接代理 |
| Anthropic 格式 | → Gemini | 转换后调用 Gemini |
| Gemini 格式 | → OpenAI | 转换后调用 GPT |
| Gemini 格式 | → Anthropic | 转换后调用 Claude |
| Gemini 格式 | → Gemini | 直接代理 |

### API 路由设计

```
POST /v1/chat/completions          # OpenAI 格式入口
POST /v1/messages                  # Anthropic 格式入口
POST /v1/models/{model}:generateContent  # Gemini 格式入口
```

通过 `model` 参数路由到目标服务商：
- `gpt-*`, `o1-*` → OpenAI
- `claude-*` → Anthropic
- `gemini-*` → Gemini

### 三种格式对比

#### 请求格式对比

**OpenAI 格式:**
```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "You are helpful."},
    {"role": "user", "content": "Hello"}
  ],
  "max_tokens": 1000,
  "temperature": 0.7,
  "stream": true
}
```

**Anthropic 格式:**
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "system": "You are helpful.",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "max_tokens": 1000,
  "temperature": 0.7,
  "stream": true
}
```

**Gemini 格式:**
```json
{
  "contents": [
    {"role": "user", "parts": [{"text": "Hello"}]}
  ],
  "systemInstruction": {
    "parts": [{"text": "You are helpful."}]
  },
  "generationConfig": {
    "maxOutputTokens": 1000,
    "temperature": 0.7
  }
}
```

#### 响应格式对比

**OpenAI 格式:**
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "model": "gpt-4",
  "choices": [{
    "index": 0,
    "message": {"role": "assistant", "content": "Hi!"},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
}
```

**Anthropic 格式:**
```json
{
  "id": "msg_xxx",
  "type": "message",
  "model": "claude-3-5-sonnet-20241022",
  "role": "assistant",
  "content": [{"type": "text", "text": "Hi!"}],
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 10, "output_tokens": 5}
}
```

**Gemini 格式:**
```json
{
  "candidates": [{
    "content": {"parts": [{"text": "Hi!"}], "role": "model"},
    "finishReason": "STOP"
  }],
  "usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 5}
}
```

### 转换规则

#### Messages 转换

| 源格式 | 字段 | 目标格式 | 字段 |
|--------|------|----------|------|
| OpenAI | messages[role=system] | Anthropic | system |
| OpenAI | messages[role=system] | Gemini | systemInstruction |
| OpenAI | messages[].content | Anthropic | messages[].content |
| OpenAI | messages[].content | Gemini | contents[].parts[].text |
| Anthropic | system | OpenAI | messages[0] (role=system) |
| Anthropic | messages | OpenAI | messages |

#### 参数映射

| OpenAI | Anthropic | Gemini |
|--------|-----------|--------|
| max_tokens | max_tokens | generationConfig.maxOutputTokens |
| temperature | temperature | generationConfig.temperature |
| top_p | top_p | generationConfig.topP |
| stop | stop_sequences | generationConfig.stopSequences |
| stream | stream | (URL 参数 alt=sse) |

#### Finish Reason 映射

| OpenAI | Anthropic | Gemini |
|--------|-----------|--------|
| stop | end_turn | STOP |
| length | max_tokens | MAX_TOKENS |
| tool_calls | tool_use | - |
| content_filter | - | SAFETY |

---

## 项目结构设计

```
ai_gateway/
├── app/
│   ├── __init__.py
│   ├── main.py                 # FastAPI 应用入口
│   ├── config.py               # 配置管理
│   ├── routers/
│   │   ├── __init__.py
│   │   ├── openai.py           # OpenAI 格式路由
│   │   ├── anthropic.py        # Anthropic 格式路由
│   │   └── gemini.py           # Gemini 格式路由
│   ├── adapters/
│   │   ├── __init__.py
│   │   ├── base.py             # 适配器基类
│   │   ├── openai_adapter.py   # OpenAI 适配器
│   │   ├── anthropic_adapter.py# Anthropic 适配器
│   │   └── gemini_adapter.py   # Gemini 适配器
│   ├── converters/
│   │   ├── __init__.py
│   │   ├── openai_to_anthropic.py
│   │   ├── openai_to_gemini.py
│   │   ├── anthropic_to_openai.py
│   │   ├── anthropic_to_gemini.py
│   │   ├── gemini_to_openai.py
│   │   └── gemini_to_anthropic.py
│   ├── models/
│   │   ├── __init__.py
│   │   ├── openai.py           # OpenAI 请求/响应模型
│   │   ├── anthropic.py        # Anthropic 请求/响应模型
│   │   └── gemini.py           # Gemini 请求/响应模型
│   └── utils/
│       ├── __init__.py
│       ├── http_client.py      # httpx 封装
│       └── stream.py           # 流式响应处理
├── tests/
│   ├── test_converters/
│   └── test_adapters/
├── docs/
├── requirements.txt
├── Dockerfile
└── README.md
```

---

## 待确认事项

1. **模型名映射**: 是否需要支持别名？如 `gpt4` → `gpt-4`
2. ~~多模态支持~~: 暂不支持
3. ~~Tool/Function Calling~~: 需要支持
4. **日志/监控**: 是否需要请求日志和监控？

---

## Tool/Function Calling 转换设计

### 三种格式对比

#### 定义工具

**OpenAI 格式:**
```json
{
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "description": "Get current weather",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string", "description": "City name"}
        },
        "required": ["location"]
      }
    }
  }],
  "tool_choice": "auto"
}
```

**Anthropic 格式:**
```json
{
  "tools": [{
    "name": "get_weather",
    "description": "Get current weather",
    "input_schema": {
      "type": "object",
      "properties": {
        "location": {"type": "string", "description": "City name"}
      },
      "required": ["location"]
    }
  }],
  "tool_choice": {"type": "auto"}
}
```

**Gemini 格式:**
```json
{
  "tools": [{
    "functionDeclarations": [{
      "name": "get_weather",
      "description": "Get current weather",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string", "description": "City name"}
        },
        "required": ["location"]
      }
    }]
  }]
}
```

#### 调用工具（模型响应）

**OpenAI 格式:**
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "id": "call_xxx",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": "{\"location\": \"Tokyo\"}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }]
}
```

**Anthropic 格式:**
```json
{
  "content": [{
    "type": "tool_use",
    "id": "toolu_xxx",
    "name": "get_weather",
    "input": {"location": "Tokyo"}
  }],
  "stop_reason": "tool_use"
}
```

**Gemini 格式:**
```json
{
  "candidates": [{
    "content": {
      "parts": [{
        "functionCall": {
          "name": "get_weather",
          "args": {"location": "Tokyo"}
        }
      }]
    },
    "finishReason": "STOP"
  }]
}
```

#### 工具结果（用户提交）

**OpenAI 格式:**
```json
{
  "messages": [
    {"role": "assistant", "tool_calls": [...]},
    {
      "role": "tool",
      "tool_call_id": "call_xxx",
      "content": "{\"temp\": 20}"
    }
  ]
}
```

**Anthropic 格式:**
```json
{
  "messages": [
    {"role": "assistant", "content": [{"type": "tool_use", ...}]},
    {
      "role": "user",
      "content": [{
        "type": "tool_result",
        "tool_use_id": "toolu_xxx",
        "content": "{\"temp\": 20}"
      }]
    }
  ]
}
```

**Gemini 格式:**
```json
{
  "contents": [
    {"role": "model", "parts": [{"functionCall": {...}}]},
    {
      "role": "user",
      "parts": [{
        "functionResponse": {
          "name": "get_weather",
          "response": {"temp": 20}
        }
      }]
    }
  ]
}
```

### Tool Calling 转换映射

| 字段 | OpenAI | Anthropic | Gemini |
|------|--------|-----------|--------|
| 工具定义 | tools[].function | tools[] | tools[].functionDeclarations[] |
| 参数 Schema | function.parameters | input_schema | parameters |
| 工具调用 ID | tool_calls[].id | content[].id | (无 ID，按 name 匹配) |
| 调用参数 | function.arguments (string) | input (object) | args (object) |
| 结果角色 | role: "tool" | role: "user" + type: "tool_result" | role: "user" + functionResponse |
| 结束原因 | tool_calls | tool_use | STOP |

### 转换注意事项

1. **参数格式**: OpenAI 的 arguments 是 JSON 字符串，Anthropic/Gemini 是对象
2. **ID 处理**: Gemini 无调用 ID，需生成或按 name 匹配
3. **多工具调用**: 三种格式都支持一次调用多个工具
4. **流式 Tool Call**: 需要处理 delta 中的增量 tool_calls
