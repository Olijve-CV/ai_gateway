"""Convert Anthropic format to Gemini format."""

import json
from typing import Any

from py.app.models import anthropic as ant
from py.app.models import gemini as gem


def convert_request(req: ant.MessagesRequest) -> tuple[gem.GenerateContentRequest, str]:
    """Convert Anthropic messages request to Gemini generate content request.

    Returns (request, model_name) tuple.
    """
    # System instruction
    system_instruction = None
    if req.system:
        system_instruction = gem.SystemInstruction(
            parts=[gem.TextPart(text=req.system)]
        )

    # Convert messages
    contents: list[gem.Content] = []

    for msg in req.messages:
        if msg.role == "user":
            if isinstance(msg.content, str):
                contents.append(
                    gem.Content(role="user", parts=[gem.TextPart(text=msg.content)])
                )
            else:
                parts: list[gem.Part] = []
                for block in msg.content:
                    if isinstance(block, ant.TextContent):
                        parts.append(gem.TextPart(text=block.text))
                    elif isinstance(block, ant.ToolResultContent):
                        content = (
                            block.content
                            if isinstance(block.content, str)
                            else block.content[0].text
                        )
                        # Need to find the tool name - we'll use a placeholder
                        parts.append(
                            gem.FunctionResponsePart(
                                functionResponse={
                                    "name": "",  # Will need context to fill this
                                    "response": json.loads(content)
                                    if content.startswith("{")
                                    else {"result": content},
                                }
                            )
                        )
                if parts:
                    contents.append(gem.Content(role="user", parts=parts))

        elif msg.role == "assistant":
            if isinstance(msg.content, str):
                contents.append(
                    gem.Content(role="model", parts=[gem.TextPart(text=msg.content)])
                )
            else:
                parts = []
                for block in msg.content:
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
                if parts:
                    contents.append(gem.Content(role="model", parts=parts))

    # Convert tools
    tools = None
    if req.tools:
        function_declarations = []
        for tool in req.tools:
            function_declarations.append(
                gem.FunctionDeclaration(
                    name=tool.name,
                    description=tool.description,
                    parameters={
                        "type": tool.input_schema.type,
                        "properties": tool.input_schema.properties,
                        "required": tool.input_schema.required,
                    },
                )
            )
        tools = [gem.ToolConfig(functionDeclarations=function_declarations)]

    # Generation config
    generation_config = gem.GenerationConfig(
        temperature=req.temperature,
        topP=req.top_p,
        topK=req.top_k,
        maxOutputTokens=req.max_tokens,
        stopSequences=req.stop_sequences,
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


def convert_response(resp: gem.GenerateContentResponse, model: str) -> ant.MessagesResponse:
    """Convert Gemini generate content response to Anthropic messages response."""
    content: list[ant.TextContent | ant.ToolUseContent] = []

    if resp.candidates:
        candidate = resp.candidates[0]
        for i, part in enumerate(candidate.content.parts):
            if isinstance(part, gem.TextPart):
                content.append(ant.TextContent(text=part.text))
            elif isinstance(part, gem.FunctionCallPart):
                fc = part.functionCall
                content.append(
                    ant.ToolUseContent(
                        id=f"toolu_{i}",
                        name=fc.get("name", ""),
                        input=fc.get("args", {}),
                    )
                )

    # Map finish reason
    finish_reason = resp.candidates[0].finishReason if resp.candidates else "STOP"
    stop_reason_map = {
        "STOP": "end_turn",
        "MAX_TOKENS": "max_tokens",
        "SAFETY": "end_turn",
        "RECITATION": "end_turn",
    }
    stop_reason = stop_reason_map.get(finish_reason or "STOP", "end_turn")

    # Check for tool use
    has_tool_use = any(isinstance(c, ant.ToolUseContent) for c in content)
    if has_tool_use:
        stop_reason = "tool_use"

    return ant.MessagesResponse(
        id=f"msg_gemini_{id(resp)}",
        model=model,
        content=content if content else [ant.TextContent(text="")],
        stop_reason=stop_reason,
        usage=ant.ResponseUsage(
            input_tokens=resp.usageMetadata.promptTokenCount if resp.usageMetadata else 0,
            output_tokens=resp.usageMetadata.candidatesTokenCount if resp.usageMetadata else 0,
        ),
    )


def convert_stream_chunk(data: dict[str, Any], msg_id: str) -> str | None:
    """Convert Gemini stream chunk to Anthropic stream event.

    Returns SSE formatted string or None if chunk should be skipped.
    """
    candidates = data.get("candidates", [])
    if not candidates:
        return None

    candidate = candidates[0]
    content = candidate.get("content", {})
    parts = content.get("parts", [])

    result = ""

    for part in parts:
        if "text" in part:
            event = {
                "type": "content_block_delta",
                "index": 0,
                "delta": {"type": "text_delta", "text": part["text"]},
            }
            result += f"event: content_block_delta\ndata: {json.dumps(event)}\n\n"

    finish_reason = candidate.get("finishReason")
    if finish_reason:
        stop_reason_map = {
            "STOP": "end_turn",
            "MAX_TOKENS": "max_tokens",
            "SAFETY": "end_turn",
        }
        stop_reason = stop_reason_map.get(finish_reason, "end_turn")
        event = {
            "type": "message_delta",
            "delta": {"stop_reason": stop_reason},
            "usage": {"output_tokens": 0},
        }
        result += f"event: message_delta\ndata: {json.dumps(event)}\n\n"
        result += "event: message_stop\ndata: {\"type\": \"message_stop\"}\n\n"

    return result if result else None
