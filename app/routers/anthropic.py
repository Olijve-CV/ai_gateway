"""Anthropic format API routes."""

import json
from typing import Any

from fastapi import APIRouter, Body, Depends, Header, Request
from fastapi.responses import JSONResponse, StreamingResponse
from sqlalchemy.ext.asyncio import AsyncSession

from app.adapters.anthropic_adapter import AnthropicAdapter
from app.adapters.gemini_adapter import GeminiAdapter
from app.adapters.openai_adapter import OpenAIAdapter
from app.converters import anthropic_to_gemini, anthropic_to_openai
from app.dependencies.auth import AuthResult, get_auth_context
from app.dependencies.database import get_db
from app.models.anthropic import MessagesRequest
from app.services.api_key_service import ApiKeyService
from app.services.config_service import ConfigService

router = APIRouter(prefix="/v1", tags=["Anthropic Format"])

# Example request body for Swagger documentation
MESSAGES_REQUEST_EXAMPLE = {
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [
        {"role": "user", "content": "Hello, how are you?"}
    ]
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
        return "anthropic"


def extract_api_key(authorization: str | None, x_api_key: str | None) -> str:
    """Extract API key from headers."""
    if x_api_key:
        return x_api_key
    if authorization:
        if authorization.startswith("Bearer "):
            return authorization[7:]
        return authorization
    raise ValueError("API key is required (x-api-key or Authorization header)")


@router.post("/messages")
async def messages(
    request: Request,
    req: MessagesRequest = Body(..., openapi_examples={"default": {"value": MESSAGES_REQUEST_EXAMPLE}}),
    auth: AuthResult = Depends(get_auth_context),
    db: AsyncSession = Depends(get_db),
):
    """Anthropic-compatible messages endpoint.

    Authentication:
    - API Key (recommended): Authorization: Bearer agw_xxxxx or X-API-Key: agw_xxxxx
    - JWT Token: Authorization: Bearer <jwt_token>

    Automatically routes to the correct provider based on model name:
    - gpt-*, o1, o3 → OpenAI
    - claude-* → Anthropic
    - gemini-* → Gemini
    """
    provider = get_target_provider(req.model)

    # Get provider credentials
    if auth.provider_config:
        # API Key authentication - use associated config
        if auth.provider_config.provider != provider:
            return JSONResponse(
                status_code=400,
                content={
                    "error": {
                        "type": "invalid_request_error",
                        "message": f"Model {req.model} requires {provider} provider, "
                        f"but API Key is configured for {auth.provider_config.provider}",
                    }
                },
            )
        base_url, api_key = auth.get_provider_credentials()
    else:
        # JWT authentication - use user's default config
        config_service = ConfigService(db)
        result = await config_service.get_default_config(auth.user, provider)
        if not result:
            return JSONResponse(
                status_code=400,
                content={
                    "error": {
                        "type": "invalid_request_error",
                        "message": f"No active {provider} configuration found. "
                        "Please configure your API key first.",
                    }
                },
            )
        base_url, api_key = result

    # Execute request
    if provider == "anthropic":
        response = await _handle_anthropic(req, api_key, base_url)
    elif provider == "openai":
        response = await _handle_openai(req, api_key, base_url)
    elif provider == "gemini":
        response = await _handle_gemini(req, api_key, base_url)
    else:
        response = await _handle_anthropic(req, api_key, base_url)

    # Record usage (if API Key authentication)
    if auth.api_key and hasattr(request.state, "api_key"):
        await _record_usage(db, request.state.api_key, req, response)

    return response


async def _handle_anthropic(req: MessagesRequest, api_key: str, base_url: str):
    """Handle request to Anthropic directly."""
    adapter = AnthropicAdapter(api_key, base_url)
    request_data = req.model_dump(exclude_none=True)

    if req.stream:
        async def generate():
            async for chunk in adapter.messages_stream(request_data):
                yield chunk

        return StreamingResponse(generate(), media_type="text/event-stream")
    else:
        result, status_code = await adapter.messages(request_data)
        return JSONResponse(content=result, status_code=status_code)


async def _handle_openai(req: MessagesRequest, api_key: str, base_url: str):
    """Handle request to OpenAI with format conversion."""
    adapter = OpenAIAdapter(api_key, base_url)

    openai_req = anthropic_to_openai.convert_request(req)
    request_data = openai_req.model_dump(exclude_none=True)

    if req.stream:
        async def generate():
            initial_event = {
                "type": "message_start",
                "message": {
                    "id": "msg_temp",
                    "type": "message",
                    "role": "assistant",
                    "model": req.model,
                    "content": [],
                    "stop_reason": None,
                    "usage": {"input_tokens": 0, "output_tokens": 0},
                },
            }
            yield f"event: message_start\ndata: {json.dumps(initial_event)}\n\n".encode()

            block_start = {
                "type": "content_block_start",
                "index": 0,
                "content_block": {"type": "text", "text": ""},
            }
            yield f"event: content_block_start\ndata: {json.dumps(block_start)}\n\n".encode()

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
                                converted = anthropic_to_openai.convert_stream_chunk(event)
                                if converted:
                                    yield converted.encode()
                            except json.JSONDecodeError:
                                pass

        return StreamingResponse(generate(), media_type="text/event-stream")
    else:
        result, status_code = await adapter.chat_completions(request_data)
        if status_code >= 400:
            return JSONResponse(content=result, status_code=status_code)

        from app.models.openai import ChatCompletionResponse

        openai_resp = ChatCompletionResponse(**result)
        anthropic_resp = anthropic_to_openai.convert_response(openai_resp)
        return JSONResponse(content=anthropic_resp.model_dump(exclude_none=True))


async def _handle_gemini(req: MessagesRequest, api_key: str, base_url: str):
    """Handle request to Gemini with format conversion."""
    adapter = GeminiAdapter(api_key, base_url)

    gemini_req, model = anthropic_to_gemini.convert_request(req)
    request_data = gemini_req.model_dump(exclude_none=True)
    gemini_model = req.model

    if req.stream:
        async def generate():
            msg_id = f"msg_{id(req)}"
            initial_event = {
                "type": "message_start",
                "message": {
                    "id": msg_id,
                    "type": "message",
                    "role": "assistant",
                    "model": req.model,
                    "content": [],
                    "stop_reason": None,
                    "usage": {"input_tokens": 0, "output_tokens": 0},
                },
            }
            yield f"event: message_start\ndata: {json.dumps(initial_event)}\n\n".encode()

            block_start = {
                "type": "content_block_start",
                "index": 0,
                "content_block": {"type": "text", "text": ""},
            }
            yield f"event: content_block_start\ndata: {json.dumps(block_start)}\n\n".encode()

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
                                converted = anthropic_to_gemini.convert_stream_chunk(
                                    event, msg_id
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
        anthropic_resp = anthropic_to_gemini.convert_response(gemini_resp, req.model)
        return JSONResponse(content=anthropic_resp.model_dump(exclude_none=True))


async def _record_usage(
    db: AsyncSession,
    api_key,
    req: MessagesRequest,
    response,
) -> None:
    """Record API usage."""
    service = ApiKeyService(db)

    # Extract token usage (from response)
    prompt_tokens = 0
    completion_tokens = 0
    status_code = 200

    if isinstance(response, JSONResponse):
        # Non-streaming response, can get usage directly
        try:
            body = response.body.decode()
            data = json.loads(body)
            usage = data.get("usage", {})
            prompt_tokens = usage.get("input_tokens", 0)
            completion_tokens = usage.get("output_tokens", 0)
        except Exception:
            pass

    await service.record_usage(
        api_key=api_key,
        endpoint="/v1/messages",
        model=req.model,
        prompt_tokens=prompt_tokens,
        completion_tokens=completion_tokens,
        status_code=status_code,
    )
