# 快速开始

## 环境要求

- Python 3.10+ / Node.js 18+ / Go 1.21+
- 有效的 AI 服务商 API Key

## 安装

```bash
# 克隆项目
git clone https://github.com/Olijve-CV/ai_gateway.git
cd ai_gateway

# 安装依赖
pip install -r requirements.txt
```

## 配置

### 1. 创建配置文件

复制示例配置文件:
```bash
cp config/config.example.yaml config/config.yaml
```

### 2. 配置 API Keys

编辑 `config/config.yaml`:
```yaml
server:
  host: 0.0.0.0
  port: 8080

adapters:
  openai:
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1

  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    base_url: https://api.anthropic.com/v1

  gemini:
    api_key: ${GEMINI_API_KEY}
    base_url: https://generativelanguage.googleapis.com/v1
```

### 3. 设置环境变量

```bash
export OPENAI_API_KEY="sk-xxx"
export ANTHROPIC_API_KEY="sk-ant-xxx"
export GEMINI_API_KEY="xxx"
```

## 启动服务

```bash
# 开发模式
python main.py

# 或使用 uvicorn
uvicorn main:app --host 0.0.0.0 --port 8080 --reload
```

服务启动后访问: `http://localhost:8080`

## 验证安装

### 测试 GPT 接口
```bash
curl -X POST http://localhost:8080/v1/gpt/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Say hello"}]
  }'
```

### 测试 Claude 接口
```bash
curl -X POST http://localhost:8080/v1/anthropic/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Say hello"}]
  }'
```

### 测试 Gemini 接口
```bash
curl -X POST http://localhost:8080/v1/gemini/generateContent \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-pro",
    "contents": [{"role": "user", "parts": [{"text": "Say hello"}]}]
  }'
```

## 下一步

- 阅读 [架构设计](architecture.md) 了解系统设计
- 查看 [API 参考](api-reference.md) 了解完整接口文档
