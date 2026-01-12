"""Gemini format API routes."""

import json

from fastapi import APIRouter, Body, Depends, Query, Request
from fastapi.responses import JSONResponse, StreamingResponse
from sqlalchemy.ext.asyncio import AsyncSession

from py.app.adapters.anthropic_adapter import AnthropicAdapter
from py.app.adapters.gemini_adapter import GeminiAdapter
from py.app.adapters.openai_adapter import OpenAIAdapter
from py.app.converters import gemini_to_anthropic, gemini_to_openai
from py.app.dependencies.auth import AuthResult, get_auth_context
from py.app.dependencies.database import get_db
from py.app.models.gemini import GenerateContentRequest
from py.app.services.api_key_service import ApiKeyService
from py.app.services.config_service import ConfigService

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
    request: Request,
    model: str,
    req: GenerateContentRequest = Body(..., openapi_examples={"default": {"value": GENERATE_CONTENT_EXAMPLE}}),
    auth: AuthResult = Depends(get_auth_context),
    db: AsyncSession = Depends(get_db),
):
    """Gemini-compatible generateContent endpoint.

    Authentication:
    - API Key (recommended): Authorization: Bearer agw_xxxxx or X-API-Key: agw_xxxxx
    - JWT Token: Authorization: Bearer <jwt_token>

    Automatically routes to the correct provider based on model name:
    - gpt-*, o1, o3 → OpenAI
    - claude-* → Anthropic
    - gemini-* → Gemini
    """
    provider = get_target_provider(model)

    # Get provider credentials
    if auth.provider_config:
        # API Key authentication - use associated config
        if auth.provider_config.provider != provider:
            return JSONResponse(
                status_code=400,
                content={
                    "error": {
                        "code": 400,
                        "message": f"Model {model} requires {provider} provider, "
                        f"but API Key is configured for {auth.provider_config.provider}",
                        "status": "INVALID_ARGUMENT",
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
                        "code": 400,
                        "message": f"No active {provider} configuration found. "
                        "Please configure your API key first.",
                        "status": "INVALID_ARGUMENT",
                    }
                },
            )
        base_url, api_key = result

    # Execute request
    if provider == "gemini":
        response = await _handle_gemini(req, model, api_key, base_url)
    elif provider == "openai":
        response = await _handle_openai(req, model, api_key, base_url)
    elif provider == "anthropic":
        response = await _handle_anthropic(req, model, api_key, base_url)
    else:
        response = await _handle_gemini(req, model, api_key, base_url)

    # Record usage (if API Key authentication)
    if auth.api_key and hasattr(request.state, "api_key"):
        await _record_usage(db, request.state.api_key, model, response)

    return response


@router.post("/models/{model}:streamGenerateContent")
async def stream_generate_content(
    request: Request,
    model: str,
    req: GenerateContentRequest = Body(..., openapi_examples={"default": {"value": GENERATE_CONTENT_EXAMPLE}}),
    auth: AuthResult = Depends(get_auth_context),
    db: AsyncSession = Depends(get_db),
    alt: str = Query("sse", description="Response format"),
):
    """Gemini-compatible streamGenerateContent endpoint.

    Authentication:
    - API Key (recommended): Authorization: Bearer agw_xxxxx or X-API-Key: agw_xxxxx
    - JWT Token: Authorization: Bearer <jwt_token>

    Automatically routes to the correct provider based on model name:
    - gpt-*, o1, o3 → OpenAI
    - claude-* → Anthropic
    - gemini-* → Gemini
    """
    provider = get_target_provider(model)

    # Get provider credentials
    if auth.provider_config:
        # API Key authentication - use associated config
        if auth.provider_config.provider != provider:
            return JSONResponse(
                status_code=400,
                content={
                    "error": {
                        "code": 400,
                        "message": f"Model {model} requires {provider} provider, "
                        f"but API Key is configured for {auth.provider_config.provider}",
                        "status": "INVALID_ARGUMENT",
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
                        "code": 400,
                        "message": f"No active {provider} configuration found. "
                        "Please configure your API key first.",
                        "status": "INVALID_ARGUMENT",
                    }
                },
            )
        base_url, api_key = result

    # Execute streaming request
    if provider == "gemini":
        return await _handle_gemini_stream(req, model, api_key, base_url)
    elif provider == "openai":
        return await _handle_openai_stream(req, model, api_key, base_url)
    elif provider == "anthropic":
        return await _handle_anthropic_stream(req, model, api_key, base_url)
    else:
        return await _handle_gemini_stream(req, model, api_key, base_url)


async def _handle_gemini(req: GenerateContentRequest, model: str, api_key: str, base_url: str):
    """Handle request to Gemini directly."""
    adapter = GeminiAdapter(api_key, base_url)
    request_data = req.model_dump(exclude_none=True)
    result, status_code = await adapter.generate_content(model, request_data)
    return JSONResponse(content=result, status_code=status_code)


async def _handle_gemini_stream(req: GenerateContentRequest, model: str, api_key: str, base_url: str):
    """Handle streaming request to Gemini directly."""
    adapter = GeminiAdapter(api_key, base_url)
    request_data = req.model_dump(exclude_none=True)

    async def generate():
        async for chunk in adapter.generate_content_stream(model, request_data):
            yield chunk

    return StreamingResponse(generate(), media_type="text/event-stream")


async def _handle_openai(req: GenerateContentRequest, model: str, api_key: str, base_url: str):
    """Handle request to OpenAI with format conversion."""
    adapter = OpenAIAdapter(api_key, base_url)

    openai_req = gemini_to_openai.convert_request(req)
    openai_req.model = model
    request_data = openai_req.model_dump(exclude_none=True)

    result, status_code = await adapter.chat_completions(request_data)
    if status_code >= 400:
        return JSONResponse(content=result, status_code=status_code)

    from py.app.models.openai import ChatCompletionResponse

    openai_resp = ChatCompletionResponse(**result)
    gemini_resp = gemini_to_openai.convert_response(openai_resp, model)
    return JSONResponse(content=gemini_resp.model_dump(exclude_none=True))


async def _handle_openai_stream(req: GenerateContentRequest, model: str, api_key: str, base_url: str):
    """Handle streaming request to OpenAI with format conversion."""
    adapter = OpenAIAdapter(api_key, base_url)

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


async def _handle_anthropic(req: GenerateContentRequest, model: str, api_key: str, base_url: str):
    """Handle request to Anthropic with format conversion."""
    adapter = AnthropicAdapter(api_key, base_url)

    anthropic_req = gemini_to_anthropic.convert_request(req)
    anthropic_req.model = model
    request_data = anthropic_req.model_dump(exclude_none=True)

    result, status_code = await adapter.messages(request_data)
    if status_code >= 400:
        return JSONResponse(content=result, status_code=status_code)

    from py.app.models.anthropic import MessagesResponse

    anthropic_resp = MessagesResponse(**result)
    gemini_resp = gemini_to_anthropic.convert_response(anthropic_resp, model)
    return JSONResponse(content=gemini_resp.model_dump(exclude_none=True))


async def _handle_anthropic_stream(req: GenerateContentRequest, model: str, api_key: str, base_url: str):
    """Handle streaming request to Anthropic with format conversion."""
    adapter = AnthropicAdapter(api_key, base_url)

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


async def _record_usage(
    db: AsyncSession,
    api_key,
    model: str,
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
            usage_metadata = data.get("usageMetadata", {})
            prompt_tokens = usage_metadata.get("promptTokenCount", 0)
            completion_tokens = usage_metadata.get("candidatesTokenCount", 0)
        except Exception:
            pass

    await service.record_usage(
        api_key=api_key,
        endpoint="/v1/models/:generateContent",
        model=model,
        prompt_tokens=prompt_tokens,
        completion_tokens=completion_tokens,
        status_code=status_code,
    )
