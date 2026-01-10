"""Gemini format API routes."""

import json

from fastapi import APIRouter, Body, Query
from fastapi.responses import JSONResponse, StreamingResponse

from app.adapters.anthropic_adapter import AnthropicAdapter
from app.adapters.gemini_adapter import GeminiAdapter
from app.adapters.openai_adapter import OpenAIAdapter
from app.converters import gemini_to_anthropic, gemini_to_openai
from app.models.gemini import GenerateContentRequest

router = APIRouter(prefix="/v1", tags=["Gemini Format"])

# Example request body for Swagger documentation
GENERATE_CONTENT_EXAMPLE = {
    "contents": [
        {
            "role": "user",
            "parts": [{"text": "Hello, how are you?"}]
        }
    ],
    "generationConfig": {
        "maxOutputTokens": 1024
    }
}


def get_target_provider(model: str) -> str:
    """Determine target provider based on model name."""
    model_lower = model.lower()
    if model_lower.startswith(("gpt-", "o1", "o3")):
        return "openai"
    elif model_lower.startswith("claude"):
        return "anthropic"
    elif model_lower.startswith("gemini"):
        return "gemini"
    else:
        return "gemini"


@router.post("/models/{model}:generateContent")
async def generate_content(
    model: str,
    req: GenerateContentRequest = Body(..., openapi_examples={"default": {"value": GENERATE_CONTENT_EXAMPLE}}),
    key: str = Query(..., description="API Key"),
):
    """Gemini-compatible generateContent endpoint.

    Automatically routes to the correct provider based on model name:
    - gpt-*, o1, o3 → OpenAI
    - claude-* → Anthropic
    - gemini-* → Gemini
    """
    provider = get_target_provider(model)

    if provider == "gemini":
        return await _handle_gemini(req, model, key)
    elif provider == "openai":
        return await _handle_openai(req, model, key)
    elif provider == "anthropic":
        return await _handle_anthropic(req, model, key)


@router.post("/models/{model}:streamGenerateContent")
async def stream_generate_content(
    model: str,
    req: GenerateContentRequest = Body(..., openapi_examples={"default": {"value": GENERATE_CONTENT_EXAMPLE}}),
    key: str = Query(..., description="API Key"),
    alt: str = Query("sse", description="Response format"),
):
    """Gemini-compatible streamGenerateContent endpoint.

    Automatically routes to the correct provider based on model name:
    - gpt-*, o1, o3 → OpenAI
    - claude-* → Anthropic
    - gemini-* → Gemini
    """
    provider = get_target_provider(model)

    if provider == "gemini":
        return await _handle_gemini_stream(req, model, key)
    elif provider == "openai":
        return await _handle_openai_stream(req, model, key)
    elif provider == "anthropic":
        return await _handle_anthropic_stream(req, model, key)


async def _handle_gemini(req: GenerateContentRequest, model: str, api_key: str):
    """Handle request to Gemini directly."""
    adapter = GeminiAdapter(api_key)
    request_data = req.model_dump(exclude_none=True)
    result, status_code = await adapter.generate_content(model, request_data)
    return JSONResponse(content=result, status_code=status_code)


async def _handle_gemini_stream(req: GenerateContentRequest, model: str, api_key: str):
    """Handle streaming request to Gemini directly."""
    adapter = GeminiAdapter(api_key)
    request_data = req.model_dump(exclude_none=True)

    async def generate():
        async for chunk in adapter.generate_content_stream(model, request_data):
            yield chunk

    return StreamingResponse(generate(), media_type="text/event-stream")


async def _handle_openai(req: GenerateContentRequest, model: str, api_key: str):
    """Handle request to OpenAI with format conversion."""
    adapter = OpenAIAdapter(api_key)

    openai_req = gemini_to_openai.convert_request(req)
    openai_req.model = model
    request_data = openai_req.model_dump(exclude_none=True)

    result, status_code = await adapter.chat_completions(request_data)
    if status_code >= 400:
        return JSONResponse(content=result, status_code=status_code)

    from app.models.openai import ChatCompletionResponse

    openai_resp = ChatCompletionResponse(**result)
    gemini_resp = gemini_to_openai.convert_response(openai_resp, model)
    return JSONResponse(content=gemini_resp.model_dump(exclude_none=True))


async def _handle_openai_stream(req: GenerateContentRequest, model: str, api_key: str):
    """Handle streaming request to OpenAI with format conversion."""
    adapter = OpenAIAdapter(api_key)

    openai_req = gemini_to_openai.convert_request(req)
    openai_req.model = model
    openai_req.stream = True
    request_data = openai_req.model_dump(exclude_none=True)

    async def generate():
        buffer = ""
        async for chunk in adapter.chat_completions_stream(request_data):
            buffer += chunk.decode("utf-8")
            while "\n\n" in buffer:
                line, buffer = buffer.split("\n\n", 1)
                if line.startswith("data: "):
                    data = line[6:]
                    if data.strip() and data.strip() != "[DONE]":
                        try:
                            event = json.loads(data)
                            converted = gemini_to_openai.convert_stream_chunk(event)
                            if converted:
                                yield f"data: {json.dumps(converted)}\n\n".encode()
                        except json.JSONDecodeError:
                            pass

    return StreamingResponse(generate(), media_type="text/event-stream")


async def _handle_anthropic(req: GenerateContentRequest, model: str, api_key: str):
    """Handle request to Anthropic with format conversion."""
    adapter = AnthropicAdapter(api_key)

    anthropic_req = gemini_to_anthropic.convert_request(req)
    anthropic_req.model = model
    request_data = anthropic_req.model_dump(exclude_none=True)

    result, status_code = await adapter.messages(request_data)
    if status_code >= 400:
        return JSONResponse(content=result, status_code=status_code)

    from app.models.anthropic import MessagesResponse

    anthropic_resp = MessagesResponse(**result)
    gemini_resp = gemini_to_anthropic.convert_response(anthropic_resp, model)
    return JSONResponse(content=gemini_resp.model_dump(exclude_none=True))


async def _handle_anthropic_stream(req: GenerateContentRequest, model: str, api_key: str):
    """Handle streaming request to Anthropic with format conversion."""
    adapter = AnthropicAdapter(api_key)

    anthropic_req = gemini_to_anthropic.convert_request(req)
    anthropic_req.model = model
    anthropic_req.stream = True
    request_data = anthropic_req.model_dump(exclude_none=True)

    async def generate():
        buffer = ""
        async for chunk in adapter.messages_stream(request_data):
            buffer += chunk.decode("utf-8")
            while "\n\n" in buffer:
                event_str, buffer = buffer.split("\n\n", 1)
                for line in event_str.split("\n"):
                    if line.startswith("data: "):
                        data = line[6:]
                        if data.strip():
                            try:
                                event = json.loads(data)
                                converted = gemini_to_anthropic.convert_stream_event(event)
                                if converted:
                                    yield f"data: {json.dumps(converted)}\n\n".encode()
                            except json.JSONDecodeError:
                                pass

    return StreamingResponse(generate(), media_type="text/event-stream")
