"""Convert OpenAI format to Gemini format."""

import json
from typing import Any

from py.app.models import gemini as gem
from py.app.models import openai as oai


def convert_request(req: oai.ChatCompletionRequest) -> tuple[gem.GenerateContentRequest, str]:
    """Convert OpenAI chat completion request to Gemini generate content request.

    Returns (request, model_name) tuple.
    """
    # Extract system instruction
    system_instruction = None
    contents: list[gem.Content] = []

    for msg in req.messages:
        if msg.role == "system":
            system_instruction = gem.SystemInstruction(
                parts=[gem.TextPart(text=msg.content or "")]
            )
        elif msg.role == "user":
            contents.append(
                gem.Content(role="user", parts=[gem.TextPart(text=msg.content or "")])
            )
        elif msg.role == "assistant":
            if msg.tool_calls:
                parts: list[gem.Part] = []
                if msg.content:
                    parts.append(gem.TextPart(text=msg.content))
                for tc in msg.tool_calls:
                    parts.append(
                        gem.FunctionCallPart(
                            functionCall={
                                "name": tc.function.name,
                                "args": json.loads(tc.function.arguments),
                            }
                        )
                    )
                contents.append(gem.Content(role="model", parts=parts))
            else:
                contents.append(
                    gem.Content(
                        role="model", parts=[gem.TextPart(text=msg.content or "")]
                    )
                )
        elif msg.role == "tool":
            # Tool result in Gemini format
            contents.append(
                gem.Content(
                    role="user",
                    parts=[
                        gem.FunctionResponsePart(
                            functionResponse={
                                "name": msg.name or "",
                                "response": json.loads(msg.content or "{}"),
                            }
                        )
                    ],
                )
            )

    # Convert tools
    tools = None
    if req.tools:
        function_declarations = []
        for tool in req.tools:
            function_declarations.append(
                gem.FunctionDeclaration(
                    name=tool.function.name,
                    description=tool.function.description,
                    parameters={
                        "type": tool.function.parameters.type
                        if tool.function.parameters
                        else "object",
                        "properties": tool.function.parameters.properties
                        if tool.function.parameters
                        else {},
                        "required": tool.function.parameters.required
                        if tool.function.parameters
                        else [],
                    }
                    if tool.function.parameters
                    else None,
                )
            )
        tools = [gem.ToolConfig(functionDeclarations=function_declarations)]

    # Generation config
    generation_config = gem.GenerationConfig(
        temperature=req.temperature,
        topP=req.top_p,
        maxOutputTokens=req.max_tokens,
        stopSequences=req.stop if isinstance(req.stop, list) else [req.stop] if req.stop else None,
    )

    return (
        gem.GenerateContentRequest(
            contents=contents,
            systemInstruction=system_instruction,
            generationConfig=generation_config,
            tools=tools,
        ),
        req.model,
    )


def convert_response(
    resp: gem.GenerateContentResponse, model: str
) -> oai.ChatCompletionResponse:
    """Convert Gemini generate content response to OpenAI chat completion response."""
    import time

    if not resp.candidates:
        return oai.ChatCompletionResponse(
            id=f"gemini-{int(time.time())}",
            created=int(time.time()),
            model=model,
            choices=[],
            usage=None,
        )

    candidate = resp.candidates[0]
    content = None
    tool_calls = None

    text_parts = []
    function_call_parts = []

    for part in candidate.content.parts:
        if isinstance(part, gem.TextPart):
            text_parts.append(part.text)
        elif isinstance(part, gem.FunctionCallPart):
            function_call_parts.append(part.functionCall)

    if text_parts:
        content = "".join(text_parts)

    if function_call_parts:
        tool_calls = []
        for i, fc in enumerate(function_call_parts):
            tool_calls.append(
                oai.ToolCall(
                    id=f"call_{i}",
                    type="function",
                    function=oai.FunctionCall(
                        name=fc.get("name", ""),
                        arguments=json.dumps(fc.get("args", {})),
                    ),
                )
            )

    # Map finish reason
    finish_reason_map = {
        "STOP": "stop",
        "MAX_TOKENS": "length",
        "SAFETY": "content_filter",
        "RECITATION": "content_filter",
    }
    finish_reason = finish_reason_map.get(candidate.finishReason or "STOP", "stop")
    if tool_calls:
        finish_reason = "tool_calls"

    # Usage
    usage = None
    if resp.usageMetadata:
        usage = oai.Usage(
            prompt_tokens=resp.usageMetadata.promptTokenCount or 0,
            completion_tokens=resp.usageMetadata.candidatesTokenCount or 0,
            total_tokens=resp.usageMetadata.totalTokenCount or 0,
        )

    return oai.ChatCompletionResponse(
        id=f"gemini-{int(time.time())}",
        created=int(time.time()),
        model=model,
        choices=[
            oai.Choice(
                index=0,
                message=oai.ChoiceMessage(
                    role="assistant",
                    content=content,
                    tool_calls=tool_calls,
                ),
                finish_reason=finish_reason,
            )
        ],
        usage=usage,
    )


def convert_stream_chunk(data: dict[str, Any], model: str) -> str | None:
    """Convert Gemini stream chunk to OpenAI stream chunk.

    Returns SSE formatted string or None if chunk should be skipped.
    """
    import time

    candidates = data.get("candidates", [])
    if not candidates:
        return None

    candidate = candidates[0]
    content = candidate.get("content", {})
    parts = content.get("parts", [])

    text_content = ""
    for part in parts:
        if "text" in part:
            text_content += part["text"]

    if text_content:
        chunk = oai.ChatCompletionChunk(
            id=f"gemini-{int(time.time())}",
            created=int(time.time()),
            model=model,
            choices=[
                oai.StreamChoice(
                    index=0,
                    delta=oai.DeltaMessage(content=text_content),
                    finish_reason=None,
                )
            ],
        )
        return f"data: {chunk.model_dump_json()}\n\n"

    finish_reason = candidate.get("finishReason")
    if finish_reason:
        finish_reason_map = {
            "STOP": "stop",
            "MAX_TOKENS": "length",
            "SAFETY": "content_filter",
        }
        chunk = oai.ChatCompletionChunk(
            id=f"gemini-{int(time.time())}",
            created=int(time.time()),
            model=model,
            choices=[
                oai.StreamChoice(
                    index=0,
                    delta=oai.DeltaMessage(),
                    finish_reason=finish_reason_map.get(finish_reason, "stop"),
                )
            ],
        )
        return f"data: {chunk.model_dump_json()}\n\ndata: [DONE]\n\n"

    return None
