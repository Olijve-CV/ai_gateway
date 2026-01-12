"""OpenAI format API routes."""

import json

from fastapi import APIRouter, Body, Depends, Header, Request
from fastapi.responses import JSONResponse, StreamingResponse
from sqlalchemy.ext.asyncio import AsyncSession

from py.app.adapters.anthropic_adapter import AnthropicAdapter
from py.app.adapters.gemini_adapter import GeminiAdapter
from py.app.adapters.openai_adapter import OpenAIAdapter
from py.app.converters import openai_to_anthropic, openai_to_gemini
from py.app.dependencies.auth import AuthResult, get_auth_context
from py.app.dependencies.database import get_db
from py.app.models.openai import ChatCompletionRequest
from py.app.services.api_key_service import ApiKeyService
from py.app.services.config_service import ConfigService

router = APIRouter(prefix="/v1", tags=["OpenAI Format"])

# Example request body for Swagger documentation
CHAT_COMPLETION_EXAMPLE = {
    "model": "gpt-4o",
    "messages": [
        {"role": "user", "content": "Hello, how are you?"}
    ],
    "max_tokens": 1024
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
    req: ChatCompletionRequest = Body(..., openapi_examples={"default": {"value": CHAT_COMPLETION_EXAMPLE}}),
    auth: AuthResult = Depends(get_auth_context),
    db: AsyncSession = Depends(get_db),
):
    """OpenAI-compatible chat completions endpoint.

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
                        "message": f"Model {req.model} requires {provider} provider, "
                        f"but API Key is configured for {auth.provider_config.provider}",
                        "type": "invalid_request_error",
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
                        "message": f"No active {provider} configuration found. "
                        "Please configure your API key first.",
                        "type": "invalid_request_error",
                    }
                },
            )
        base_url, api_key = result

    # Execute request
    if provider == "openai":
        response = await _handle_openai(req, api_key, base_url)
    elif provider == "anthropic":
        response = await _handle_anthropic(req, api_key, base_url)
    elif provider == "gemini":
        response = await _handle_gemini(req, api_key, base_url)
    else:
        response = await _handle_openai(req, api_key, base_url)

    # Record usage (if API Key authentication)
    if auth.api_key and hasattr(request.state, "api_key"):
        await _record_usage(db, request.state.api_key, req, response)

    return response


async def _handle_openai(req: ChatCompletionRequest, api_key: str, base_url: str):
    """Handle request to OpenAI."""
    adapter = OpenAIAdapter(api_key, base_url)

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


async def _handle_anthropic(req: ChatCompletionRequest, api_key: str, base_url: str):
    """Handle request to Anthropic with format conversion."""
    adapter = AnthropicAdapter(api_key, base_url)

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

        from py.app.models.anthropic import MessagesResponse

        anthropic_resp = MessagesResponse(**result)
        openai_resp = openai_to_anthropic.convert_response(anthropic_resp, req.model)
        return JSONResponse(content=openai_resp.model_dump(exclude_none=True))


async def _handle_gemini(req: ChatCompletionRequest, api_key: str, base_url: str):
    """Handle request to Gemini with format conversion."""
    adapter = GeminiAdapter(api_key, base_url)

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

        from py.app.models.gemini import GenerateContentResponse

        gemini_resp = GenerateContentResponse(**result)
        openai_resp = openai_to_gemini.convert_response(gemini_resp, req.model)
        return JSONResponse(content=openai_resp.model_dump(exclude_none=True))


async def _record_usage(
    db: AsyncSession,
    api_key,
    req: ChatCompletionRequest,
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
            prompt_tokens = usage.get("prompt_tokens", 0)
            completion_tokens = usage.get("completion_tokens", 0)
        except Exception:
            pass

    await service.record_usage(
        api_key=api_key,
        endpoint="/v1/chat/completions",
        model=req.model,
        prompt_tokens=prompt_tokens,
        completion_tokens=completion_tokens,
        status_code=status_code,
    )
