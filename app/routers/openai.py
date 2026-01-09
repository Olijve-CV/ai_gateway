"""OpenAI format API routes."""

import json

from fastapi import APIRouter, Header, Request
from fastapi.responses import JSONResponse, StreamingResponse

from app.adapters.anthropic_adapter import AnthropicAdapter
from app.adapters.gemini_adapter import GeminiAdapter
from app.adapters.openai_adapter import OpenAIAdapter
from app.converters import openai_to_anthropic, openai_to_gemini
from app.models.openai import ChatCompletionRequest

router = APIRouter(prefix="/v1", tags=["OpenAI Format"])


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
        return "openai"


def extract_api_key(authorization: str | None) -> str:
    """Extract API key from Authorization header."""
    if not authorization:
        raise ValueError("Authorization header is required")
    if authorization.startswith("Bearer "):
        return authorization[7:]
    return authorization


@router.post("/chat/completions")
async def chat_completions(
    request: Request,
    authorization: str | None = Header(None),
):
    """OpenAI-compatible chat completions endpoint."""
    body = await request.json()
    req = ChatCompletionRequest(**body)
    api_key = extract_api_key(authorization)
    provider = get_target_provider(req.model)

    if provider == "openai":
        return await _handle_openai(req, api_key)
    elif provider == "anthropic":
        return await _handle_anthropic(req, api_key)
    elif provider == "gemini":
        return await _handle_gemini(req, api_key)


async def _handle_openai(req: ChatCompletionRequest, api_key: str):
    """Handle request to OpenAI."""
    adapter = OpenAIAdapter(api_key)

    if req.stream:
        async def generate():
            async for chunk in adapter.chat_completions_stream(
                req.model_dump(exclude_none=True)
            ):
                yield chunk

        return StreamingResponse(generate(), media_type="text/event-stream")
    else:
        result, status_code = await adapter.chat_completions(
            req.model_dump(exclude_none=True)
        )
        return JSONResponse(content=result, status_code=status_code)


async def _handle_anthropic(req: ChatCompletionRequest, api_key: str):
    """Handle request to Anthropic with format conversion."""
    adapter = AnthropicAdapter(api_key)

    anthropic_req = openai_to_anthropic.convert_request(req)
    request_data = anthropic_req.model_dump(exclude_none=True)

    if req.stream:
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
                                    converted = openai_to_anthropic.convert_stream_event(
                                        event, req.model
                                    )
                                    if converted:
                                        yield converted.encode()
                                except json.JSONDecodeError:
                                    pass

        return StreamingResponse(generate(), media_type="text/event-stream")
    else:
        result, status_code = await adapter.messages(request_data)
        if status_code >= 400:
            return JSONResponse(content=result, status_code=status_code)

        from app.models.anthropic import MessagesResponse

        anthropic_resp = MessagesResponse(**result)
        openai_resp = openai_to_anthropic.convert_response(anthropic_resp, req.model)
        return JSONResponse(content=openai_resp.model_dump(exclude_none=True))


async def _handle_gemini(req: ChatCompletionRequest, api_key: str):
    """Handle request to Gemini with format conversion."""
    adapter = GeminiAdapter(api_key)

    gemini_req, model = openai_to_gemini.convert_request(req)
    request_data = gemini_req.model_dump(exclude_none=True)
    gemini_model = req.model

    if req.stream:
        async def generate():
            buffer = ""
            async for chunk in adapter.generate_content_stream(gemini_model, request_data):
                buffer += chunk.decode("utf-8")
                while "\n" in buffer:
                    line, buffer = buffer.split("\n", 1)
                    if line.startswith("data: "):
                        data = line[6:]
                        if data.strip():
                            try:
                                event = json.loads(data)
                                converted = openai_to_gemini.convert_stream_chunk(
                                    event, req.model
                                )
                                if converted:
                                    yield converted.encode()
                            except json.JSONDecodeError:
                                pass

        return StreamingResponse(generate(), media_type="text/event-stream")
    else:
        result, status_code = await adapter.generate_content(gemini_model, request_data)
        if status_code >= 400:
            return JSONResponse(content=result, status_code=status_code)

        from app.models.gemini import GenerateContentResponse

        gemini_resp = GenerateContentResponse(**result)
        openai_resp = openai_to_gemini.convert_response(gemini_resp, req.model)
        return JSONResponse(content=openai_resp.model_dump(exclude_none=True))
