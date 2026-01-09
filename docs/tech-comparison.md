# 技术方案对比

## 需求概述

- **功能**: 统一入口代理 + API 格式互转
- **流式支持**: 需要支持 SSE 流式响应
- **目标**: 支持 OpenAI、Anthropic、Gemini 三种 API 的代理和格式转换

---

## 方案对比

### 1. Python + FastAPI

| 维度 | 评估 |
|------|------|
| **性能** | 中等，基于 uvicorn 异步，单实例 QPS 约 1000-3000 |
| **开发效率** | 高，代码简洁，类型提示完善 |
| **SDK 支持** | 优秀，openai/anthropic/google-generativeai 官方 SDK 齐全 |
| **异步支持** | 原生 async/await，httpx 异步请求 |
| **流式支持** | StreamingResponse + httpx 流式，实现简单 |
| **部署** | 简单，Docker/uvicorn/gunicorn |
| **学习成本** | 低 |

**优势:**
- 官方 SDK 最完善，API 变更跟进快
- Pydantic 模型验证，自动生成 OpenAPI 文档
- 异步性能足够应对大多数场景
- 社区资源丰富，调试方便

**劣势:**
- 相比 Go 内存占用较高
- GIL 限制 CPU 密集场景（但本项目主要是 IO 密集）

**代码示例:**
```python
from fastapi import FastAPI, Request
from pydantic import BaseModel
import httpx

app = FastAPI()

class ChatRequest(BaseModel):
    model: str
    messages: list
    max_tokens: int = 1000

@app.post("/v1/gpt/chat/completions")
async def gpt_completions(req: ChatRequest):
    async with httpx.AsyncClient() as client:
        resp = await client.post(
            "https://api.openai.com/v1/chat/completions",
            json=req.model_dump(),
            headers={"Authorization": f"Bearer {OPENAI_KEY}"}
        )
    return resp.json()
```

---

### 2. Node.js + Express

| 维度 | 评估 |
|------|------|
| **性能** | 中等，事件循环非阻塞，单实例 QPS 约 2000-5000 |
| **开发效率** | 中等，需要手动处理类型（或用 TypeScript）|
| **SDK 支持** | 良好，openai/anthropic SDK 完善，Gemini SDK 一般 |
| **异步支持** | 原生 Promise/async-await |
| **流式支持** | res.write() + EventSource，原生支持好 |
| **部署** | 简单，pm2/Docker |
| **学习成本** | 低 |

**优势:**
- 非阻塞 IO 天然适合代理场景
- npm 生态庞大
- 前后端同构，团队技术栈统一

**劣势:**
- 回调地狱风险（现代 async/await 已缓解）
- TypeScript 配置繁琐
- 单线程，CPU 密集任务需 worker

**代码示例:**
```typescript
import express from 'express';
import Anthropic from '@anthropic-ai/sdk';

const app = express();
const anthropic = new Anthropic();

app.post('/v1/anthropic/messages', async (req, res) => {
  const response = await anthropic.messages.create({
    model: req.body.model,
    max_tokens: req.body.max_tokens,
    messages: req.body.messages
  });
  res.json(response);
});
```

---

### 3. Go + Gin

| 维度 | 评估 |
|------|------|
| **性能** | 高，goroutine 并发，单实例 QPS 可达 10000+ |
| **开发效率** | 中等，代码量较多，但逻辑清晰 |
| **SDK 支持** | 一般，官方 SDK 较少，多为社区维护 |
| **异步支持** | goroutine 天然并发 |
| **流式支持** | Flush() 手动刷新，需自行处理 chunked 响应 |
| **部署** | 优秀，单二进制，内存占用低 |
| **学习成本** | 中等 |

**优势:**
- 性能最优，资源占用最低
- 编译型语言，部署简单（单二进制）
- 强类型，运行时错误少
- 适合高并发生产环境

**劣势:**
- AI 服务商官方 SDK 支持较弱，需手动封装 HTTP 请求
- 错误处理繁琐
- 开发速度相对较慢

**代码示例:**
```go
package main

import (
    "github.com/gin-gonic/gin"
    "net/http"
    "bytes"
    "encoding/json"
)

func main() {
    r := gin.Default()

    r.POST("/v1/gpt/chat/completions", func(c *gin.Context) {
        var req map[string]interface{}
        c.BindJSON(&req)

        body, _ := json.Marshal(req)
        resp, _ := http.Post(
            "https://api.openai.com/v1/chat/completions",
            "application/json",
            bytes.NewBuffer(body),
        )
        defer resp.Body.Close()

        var result map[string]interface{}
        json.NewDecoder(resp.Body).Decode(&result)
        c.JSON(200, result)
    })

    r.Run(":8080")
}
```

---

## 综合对比表

| 维度 | Python+FastAPI | Node.js+Express | Go+Gin |
|------|----------------|-----------------|--------|
| 性能 | ★★★☆☆ | ★★★☆☆ | ★★★★★ |
| 开发效率 | ★★★★★ | ★★★★☆ | ★★★☆☆ |
| SDK 支持 | ★★★★★ | ★★★★☆ | ★★☆☆☆ |
| 流式支持 | ★★★★★ | ★★★★★ | ★★★☆☆ |
| 类型安全 | ★★★★☆ | ★★★☆☆ (TS) | ★★★★★ |
| 部署便捷 | ★★★★☆ | ★★★★☆ | ★★★★★ |
| 内存占用 | ★★★☆☆ | ★★★☆☆ | ★★★★★ |
| 社区生态 | ★★★★★ | ★★★★★ | ★★★★☆ |

---

## 推荐方案

### 场景化推荐

| 场景 | 推荐 | 理由 |
|------|------|------|
| 快速原型/MVP | Python + FastAPI | 开发最快，SDK 支持最好 |
| 团队 JS 技术栈 | Node.js + Express | 技术栈统一，上手快 |
| 高并发生产环境 | Go + Gin | 性能最优，资源占用低 |
| 需要频繁调整 API | Python + FastAPI | 动态语言灵活，迭代快 |

### 本项目推荐: **Python + FastAPI**

**理由:**
1. **SDK 完善**: OpenAI、Anthropic、Google 三家官方 Python SDK 维护最积极
2. **流式支持好**: StreamingResponse + httpx 流式请求，代码简洁
3. **开发效率**: Pydantic 自动校验，减少样板代码
4. **格式转换便捷**: Python 字典操作灵活，适合 JSON 格式转换
5. **性能足够**: 对于 API 网关场景，瓶颈在上游 AI 服务，本地性能非核心

---

## 流式响应实现对比

### Python + FastAPI 流式实现
```python
from fastapi import FastAPI
from fastapi.responses import StreamingResponse
import httpx

@app.post("/v1/gpt/chat/completions")
async def stream_chat(req: ChatRequest):
    async def generate():
        async with httpx.AsyncClient() as client:
            async with client.stream(
                "POST",
                "https://api.openai.com/v1/chat/completions",
                json={**req.model_dump(), "stream": True},
                headers={"Authorization": f"Bearer {OPENAI_KEY}"}
            ) as resp:
                async for chunk in resp.aiter_lines():
                    if chunk:
                        yield f"{chunk}\n"

    return StreamingResponse(generate(), media_type="text/event-stream")
```

### Node.js 流式实现
```typescript
app.post('/v1/gpt/chat/completions', async (req, res) => {
  res.setHeader('Content-Type', 'text/event-stream');

  const response = await fetch('https://api.openai.com/v1/chat/completions', {
    method: 'POST',
    headers: { 'Authorization': `Bearer ${OPENAI_KEY}` },
    body: JSON.stringify({ ...req.body, stream: true })
  });

  const reader = response.body.getReader();
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    res.write(new TextDecoder().decode(value));
  }
  res.end();
});
```

### Go 流式实现
```go
func streamChat(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")

    resp, _ := http.Post(/* ... */)
    defer resp.Body.Close()

    reader := bufio.NewReader(resp.Body)
    c.Stream(func(w io.Writer) bool {
        line, err := reader.ReadBytes('\n')
        if err != nil {
            return false
        }
        w.Write(line)
        return true
    })
}
```

---

## 后续讨论点

1. **API Key 管理**: 单 Key 还是多租户？
2. **格式转换策略**: 是否需要双向转换（如 OpenAI 格式调 Claude）？
3. **错误处理**: 统一错误格式还是透传上游错误？
4. **缓存/限流**: 是否需要内置？
