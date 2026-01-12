# AI Gateway

统一的 AI API 适配器网关，支持 OpenAI GPT、Anthropic Claude、Google Gemini 三种主流 AI 服务的请求路由和格式转换。

## 功能特性

- **统一接口**: 提供标准化的 API 接口，屏蔽不同 AI 服务商的差异
- **多模型支持**: 同时支持 GPT、Claude、Gemini 三大模型系列
- **格式互转**: OpenAI / Anthropic / Gemini 三种 API 格式互相转换
- **流式响应**: 支持 SSE 流式输出
- **Tool Calling**: 支持函数调用格式转换
- **透传模式**: API Key 透传，无需在网关配置密钥

## API 端点

| 格式 | 端点 | 说明 |
|------|------|------|
| OpenAI | `POST /v1/chat/completions` | 根据 model 自动路由到对应服务商 |
| Anthropic | `POST /v1/messages` | 根据 model 自动路由到对应服务商 |
| Gemini | `POST /v1/models/{model}:generateContent` | 根据 model 自动路由到对应服务商 |

**路由规则**: 根据 model 参数自动识别目标服务商
- `gpt-*`, `o1-*` → OpenAI
- `claude-*` → Anthropic
- `gemini-*` → Gemini

## 快速开始

```bash
# 安装依赖
pip install -r requirements.txt

# 启动服务
uvicorn app.main:app --reload

# 或者
python -m app.main
```

## 使用示例

```bash
# 用 OpenAI 格式调用 Claude
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $ANTHROPIC_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# 用 Anthropic 格式调用 GPT
curl -X POST http://localhost:8080/v1/messages \
  -H "x-api-key: $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## 项目结构

```
ai_gateway/
├── app/
│   ├── main.py              # FastAPI 应用入口
│   ├── config.py            # 配置管理
│   ├── routers/             # API 路由
│   │   ├── openai.py        # OpenAI 格式端点
│   │   ├── anthropic.py     # Anthropic 格式端点
│   │   └── gemini.py        # Gemini 格式端点
│   ├── adapters/            # HTTP 适配器
│   │   ├── openai_adapter.py
│   │   ├── anthropic_adapter.py
│   │   └── gemini_adapter.py
│   ├── converters/          # 格式转换器
│   │   ├── openai_to_anthropic.py
│   │   ├── openai_to_gemini.py
│   │   ├── anthropic_to_openai.py
│   │   ├── anthropic_to_gemini.py
│   │   ├── gemini_to_openai.py
│   │   └── gemini_to_anthropic.py
│   ├── models/              # Pydantic 数据模型
│   │   ├── openai.py
│   │   ├── anthropic.py
│   │   └── gemini.py
│   └── utils/               # 工具函数
├── docs/                    # 文档
├── requirements.txt
├── Dockerfile
└── README.md
```

## 文档

- [技术方案对比](../docs/tech-comparison.md)
- [设计决策](../docs/design-decisions.md)
- [架构设计](../docs/architecture.md)
- [API 参考](../docs/api-reference.md)
- [快速开始](../docs/getting-started.md)

## License

MIT
