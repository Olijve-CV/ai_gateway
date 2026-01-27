# 自定义 Provider 功能

AI Gateway 现在支持添加自定义服务商，让用户可以集成任何兼容 OpenAI API 格式的 AI 服务。

## 功能特性

1. **自定义服务商**: 支持添加任意兼容 OpenAI API 格式的服务商
2. **多个模型代码**: 每个自定义 provider 可以配置多个模型代码
3. **自动路由**: 根据模型名称自动匹配对应的自定义 provider
4. **协议支持**: 支持 OpenAI Chat、OpenAI Responses、Anthropic、Gemini 协议格式

## 配置方法

### 通过 Web 界面配置

1. 登录 AI Gateway 管理面板
2. 进入"服务配置"页面
3. 点击"+ 添加配置"
4. 选择"自定义服务商"作为提供商
5. 填写以下信息：
   - **配置名称**: 描述性的名称，如"我的自定义AI服务"
   - **API 端点**: 服务商的 API 基础URL，如 `https://api.myservice.com/v1`
   - **Protocol**: 选择兼容的协议格式（推荐 OpenAI Chat）
   - **API Key**: 服务商提供的 API 密钥
   - **模型代码**: 每行一个模型名称，如：
     ```
     custom-model-1
     custom-model-2
     advanced-model
     ```

### 通过 API 配置

```bash
curl -X POST http://localhost:8080/api/config/providers \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "custom",
    "name": "My Custom AI Service",
    "base_url": "https://api.myservice.com/v1",
    "protocol": "openai_chat",
    "api_key": "your-api-key",
    "model_codes": ["custom-model-1", "custom-model-2"]
  }'
```

## 使用方法

配置完成后，可以直接使用自定义的模型代码调用 API：

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_GATEWAY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "custom-model-1",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## 工作原理

1. **模型匹配**: 当请求到达时，系统首先检查模型名称是否匹配预定义的模式（gpt-、claude-、gemini-）
2. **自定义查找**: 如果不匹配预定义模式，系统会在用户配置的自定义 provider 中查找匹配的模型代码
3. **协议路由**: 找到匹配的配置后，根据配置的协议格式将请求路由到相应的适配器
4. **响应转换**: 适配器处理请求并返回标准格式的响应

## 注意事项

1. **模型代码唯一性**: 确保不同自定义 provider 中的模型代码不重复，否则会使用第一个匹配的配置
2. **API 兼容性**: 自定义 provider 必须兼容所选协议的 API 格式
3. **安全性**: API 密钥会自动加密存储
4. **默认配置**: 每个类型只能有一个默认配置，自定义 provider 也不例外

## 支持的协议格式

- **openai_chat**: OpenAI Chat Completions API 格式 (`/v1/chat/completions`)
- **openai_code**: OpenAI Responses API 格式 (`/v1/responses`)
- **anthropic**: Anthropic Messages API 格式 (`/v1/messages`)
- **gemini**: Gemini Generate Content API 格式 (`/v1beta/models/generateContent`)

选择错误的协议可能导致请求失败，请根据你的服务商文档选择正确的协议。

## 故障排除

1. **模型未找到**: 检查模型代码是否正确配置，以及配置是否处于启用状态
2. **API 调用失败**: 确认 API 端点和密钥正确，以及选择的协议格式与服务商兼容
3. **响应格式错误**: 可能需要尝试不同的协议格式

## 数据库迁移

如果从旧版本升级，需要添加 `model_codes` 字段到 `provider_configs` 表：

```sql
ALTER TABLE provider_configs ADD COLUMN model_codes TEXT DEFAULT '';
```

或者运行提供的迁移文件：`migrations/001_add_model_codes.sql`