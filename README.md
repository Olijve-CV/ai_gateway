# AI Gateway

统一的 AI API 适配器网关，支持 OpenAI GPT、Anthropic Claude、Google Gemini 三种主流 AI 服务的请求路由和格式转换。

## 功能特性

- **统一接口**: 提供标准化的 API 接口，屏蔽不同 AI 服务商的差异
- **多模型支持**: 同时支持 GPT、Claude、Gemini 三大模型系列
- **请求适配**: 自动转换请求格式以匹配目标服务商的 API 规范
- **响应标准化**: 统一不同服务商的响应格式，简化客户端处理
- **灵活路由**: 支持按需路由到不同的 AI 服务

## 支持的服务商

| 服务商 | 接口路径 | 支持模型 |
|--------|----------|----------|
| OpenAI | `/v1/gpt/*` | GPT-4o, GPT-4, GPT-3.5-turbo 等 |
| Anthropic | `/v1/anthropic/*` | Claude 3.5, Claude 3 系列等 |
| Google | `/v1/gemini/*` | Gemini Pro, Gemini Ultra 等 |

## 快速开始

详见 [快速开始指南](docs/getting-started.md)

## 文档

- [架构设计](docs/architecture.md)
- [API 参考](docs/api-reference.md)
- [快速开始](docs/getting-started.md)

## License

MIT
