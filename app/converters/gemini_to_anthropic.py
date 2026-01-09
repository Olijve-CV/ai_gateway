"""Convert Gemini format to Anthropic format."""

import json
from typing import Any

from app.models import anthropic as ant
from app.models import gemini as gem


def convert_request(req: gem.GenerateContentRequest) -> ant.MessagesRequest:
    """Convert Gemini generate content request to Anthropic messages request."""
    # System instruction
    system = None
    if req.systemInstruction:
        system = "".join(p.text for p in req.systemInstruction.parts)

    # Convert contents
    messages: list[ant.Message] = []

    for content in req.contents:
        role = "user" if content.role == "user" else "assistant"

        if role == "user":
            content_blocks: list[ant.ContentBlock] = []
            for part in content.parts:
                if isinstance(part, gem.TextPart):
                    content_blocks.append(ant.TextContent(text=part.text))
                elif isinstance(part, gem.FunctionResponsePart):
                    fr = part.functionResponse
                    content_blocks.append(
                        ant.ToolResultContent(
                            tool_use_id=f"toolu_{fr.get('name', '')}",
                            content=json.dumps(fr.get("response", {})),
                        )
                    )

            if content_blocks:
                # If only text, use string content
                if len(content_blocks) == 1 and isinstance(
                    content_blocks[0], ant.TextContent
                ):
                    messages.append(
                        ant.Message(role="user", content=content_blocks[0].text)
                    )
                else:
                    messages.append(ant.Message(role="user", content=content_blocks))

        else:  # assistant
            content_blocks = []
            for i, part in enumerate(content.parts):
                if isinstance(part, gem.TextPart):
                    content_blocks.append(ant.TextContent(text=part.text))
                elif isinstance(part, gem.FunctionCallPart):
                    fc = part.functionCall
                    content_blocks.append(
                        ant.ToolUseContent(
                            id=f"toolu_{i}",
                            name=fc.get("name", ""),
                            input=fc.get("args", {}),
                        )
                    )

            if content_blocks:
                if len(content_blocks) == 1 and isinstance(
                    content_blocks[0], ant.TextContent
                ):
                    messages.append(
                        ant.Message(role="assistant", content=content_blocks[0].text)
                    )
                else:
                    messages.append(
                        ant.Message(role="assistant", content=content_blocks)
                    )

    # Convert tools
    tools = None
    if req.tools:
        tools = []
        for tool_config in req.tools:
            for fd in tool_config.functionDeclarations:
                tools.append(
                    ant.ToolDefinition(
                        name=fd.name,
                        description=fd.description,
                        input_schema=ant.ToolInputSchema(
                            type=fd.parameters.get("type", "object")
                            if fd.parameters
                            else "object",
                            properties=fd.parameters.get("properties", {})
                            if fd.parameters
                            else {},
                            required=fd.parameters.get("required", [])
                            if fd.parameters
                            else [],
                        ),
                    )
                )

    # Generation config
    gen_config = req.generationConfig
    return ant.MessagesRequest(
        model="",  # Will be set by caller
        messages=messages,
        system=system,
        max_tokens=gen_config.maxOutputTokens if gen_config else 4096,
        temperature=gen_config.temperature if gen_config else None,
        top_p=gen_config.topP if gen_config else None,
        top_k=gen_config.topK if gen_config else None,
        stop_sequences=gen_config.stopSequences if gen_config else None,
        tools=tools,
    )


def convert_response(
    resp: ant.MessagesResponse, model: str
) -> gem.GenerateContentResponse:
    """Convert Anthropic messages response to Gemini generate content response."""
    parts: list[gem.Part] = []

    for block in resp.content:
        if isinstance(block, ant.TextContent):
            parts.append(gem.TextPart(text=block.text))
        elif isinstance(block, ant.ToolUseContent):
            parts.append(
                gem.FunctionCallPart(
                    functionCall={
                        "name": block.name,
                        "args": block.input,
                    }
                )
            )

    # Map stop reason
    stop_reason_map = {
        "end_turn": "STOP",
        "stop_sequence": "STOP",
        "max_tokens": "MAX_TOKENS",
        "tool_use": "STOP",
    }
    finish_reason = stop_reason_map.get(resp.stop_reason or "end_turn", "STOP")

    return gem.GenerateContentResponse(
        candidates=[
            gem.Candidate(
                content=gem.ResponseContent(parts=parts if parts else [gem.TextPart(text="")]),
                finishReason=finish_reason,
                index=0,
            )
        ],
        usageMetadata=gem.UsageMetadata(
            promptTokenCount=resp.usage.input_tokens,
            candidatesTokenCount=resp.usage.output_tokens,
            totalTokenCount=resp.usage.input_tokens + resp.usage.output_tokens,
        ),
    )


def convert_stream_event(event: dict[str, Any]) -> dict[str, Any] | None:
    """Convert Anthropic stream event to Gemini stream chunk.

    Returns dict or None if event should be skipped.
    """
    event_type = event.get("type")

    if event_type == "content_block_delta":
        delta = event.get("delta", {})
        if delta.get("type") == "text_delta":
            return {
                "candidates": [
                    {
                        "content": {
                            "parts": [{"text": delta.get("text", "")}],
                            "role": "model",
                        },
                        "index": 0,
                    }
                ]
            }

    elif event_type == "message_delta":
        delta = event.get("delta", {})
        stop_reason = delta.get("stop_reason")
        if stop_reason:
            stop_reason_map = {
                "end_turn": "STOP",
                "stop_sequence": "STOP",
                "max_tokens": "MAX_TOKENS",
                "tool_use": "STOP",
            }
            return {
                "candidates": [
                    {
                        "content": {"parts": [], "role": "model"},
                        "finishReason": stop_reason_map.get(stop_reason, "STOP"),
                        "index": 0,
                    }
                ]
            }

    return None
