"""Convert OpenAI format to Anthropic format."""

import json
from typing import Any

from app.models import anthropic as ant
from app.models import openai as oai


def convert_request(req: oai.ChatCompletionRequest) -> ant.MessagesRequest:
    """Convert OpenAI chat completion request to Anthropic messages request."""
    # Extract system message
    system_content = None
    messages: list[ant.Message] = []

    for msg in req.messages:
        if msg.role == "system":
            system_content = msg.content
        elif msg.role == "user":
            messages.append(ant.Message(role="user", content=msg.content or ""))
        elif msg.role == "assistant":
            if msg.tool_calls:
                # Convert tool calls to tool_use content blocks
                content_blocks: list[ant.ContentBlock] = []
                if msg.content:
                    content_blocks.append(ant.TextContent(text=msg.content))
                for tc in msg.tool_calls:
                    content_blocks.append(
                        ant.ToolUseContent(
                            id=tc.id,
                            name=tc.function.name,
                            input=json.loads(tc.function.arguments),
                        )
                    )
                messages.append(ant.Message(role="assistant", content=content_blocks))
            else:
                messages.append(
                    ant.Message(role="assistant", content=msg.content or "")
                )
        elif msg.role == "tool":
            # Tool result - append to last user message or create new one
            tool_result = ant.ToolResultContent(
                tool_use_id=msg.tool_call_id or "",
                content=msg.content or "",
            )
            # In Anthropic, tool results are user messages
            messages.append(ant.Message(role="user", content=[tool_result]))

    # Convert tools
    tools = None
    if req.tools:
        tools = []
        for tool in req.tools:
            tools.append(
                ant.ToolDefinition(
                    name=tool.function.name,
                    description=tool.function.description,
                    input_schema=ant.ToolInputSchema(
                        type=tool.function.parameters.type
                        if tool.function.parameters
                        else "object",
                        properties=tool.function.parameters.properties
                        if tool.function.parameters
                        else {},
                        required=tool.function.parameters.required
                        if tool.function.parameters
                        else [],
                    ),
                )
            )

    # Convert tool_choice
    tool_choice = None
    if req.tool_choice:
        if isinstance(req.tool_choice, str):
            if req.tool_choice == "auto":
                tool_choice = ant.ToolChoice(type="auto")
            elif req.tool_choice == "required":
                tool_choice = ant.ToolChoice(type="any")
            elif req.tool_choice == "none":
                tool_choice = None
        elif isinstance(req.tool_choice, dict):
            tool_choice = ant.ToolChoice(
                type="tool", name=req.tool_choice.get("function", {}).get("name")
            )

    return ant.MessagesRequest(
        model=req.model,
        messages=messages,
        system=system_content,
        max_tokens=req.max_tokens or 4096,
        temperature=req.temperature,
        top_p=req.top_p,
        stop_sequences=req.stop if isinstance(req.stop, list) else [req.stop] if req.stop else None,
        stream=req.stream,
        tools=tools,
        tool_choice=tool_choice,
    )


def convert_response(resp: ant.MessagesResponse, model: str) -> oai.ChatCompletionResponse:
    """Convert Anthropic messages response to OpenAI chat completion response."""
    import time

    # Extract content
    content = None
    tool_calls = None

    text_parts = []
    tool_use_parts = []

    for block in resp.content:
        if isinstance(block, ant.TextContent):
            text_parts.append(block.text)
        elif isinstance(block, ant.ToolUseContent):
            tool_use_parts.append(block)

    if text_parts:
        content = "".join(text_parts)

    if tool_use_parts:
        tool_calls = []
        for tu in tool_use_parts:
            tool_calls.append(
                oai.ToolCall(
                    id=tu.id,
                    type="function",
                    function=oai.FunctionCall(
                        name=tu.name,
                        arguments=json.dumps(tu.input),
                    ),
                )
            )

    # Map finish reason
    finish_reason_map = {
        "end_turn": "stop",
        "stop_sequence": "stop",
        "max_tokens": "length",
        "tool_use": "tool_calls",
    }
    finish_reason = finish_reason_map.get(resp.stop_reason or "", "stop")

    return oai.ChatCompletionResponse(
        id=resp.id,
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
        usage=oai.Usage(
            prompt_tokens=resp.usage.input_tokens,
            completion_tokens=resp.usage.output_tokens,
            total_tokens=resp.usage.input_tokens + resp.usage.output_tokens,
        ),
    )


def convert_stream_event(event: dict[str, Any], model: str) -> str | None:
    """Convert Anthropic stream event to OpenAI stream chunk.

    Returns SSE formatted string or None if event should be skipped.
    """
    import time

    event_type = event.get("type")

    if event_type == "message_start":
        msg = event.get("message", {})
        chunk = oai.ChatCompletionChunk(
            id=msg.get("id", ""),
            created=int(time.time()),
            model=model,
            choices=[
                oai.StreamChoice(
                    index=0,
                    delta=oai.DeltaMessage(role="assistant"),
                    finish_reason=None,
                )
            ],
        )
        return f"data: {chunk.model_dump_json()}\n\n"

    elif event_type == "content_block_delta":
        delta = event.get("delta", {})
        delta_type = delta.get("type")

        if delta_type == "text_delta":
            chunk = oai.ChatCompletionChunk(
                id="",
                created=int(time.time()),
                model=model,
                choices=[
                    oai.StreamChoice(
                        index=0,
                        delta=oai.DeltaMessage(content=delta.get("text", "")),
                        finish_reason=None,
                    )
                ],
            )
            return f"data: {chunk.model_dump_json()}\n\n"

    elif event_type == "message_delta":
        delta = event.get("delta", {})
        stop_reason = delta.get("stop_reason")
        if stop_reason:
            finish_reason_map = {
                "end_turn": "stop",
                "stop_sequence": "stop",
                "max_tokens": "length",
                "tool_use": "tool_calls",
            }
            chunk = oai.ChatCompletionChunk(
                id="",
                created=int(time.time()),
                model=model,
                choices=[
                    oai.StreamChoice(
                        index=0,
                        delta=oai.DeltaMessage(),
                        finish_reason=finish_reason_map.get(stop_reason, "stop"),
                    )
                ],
            )
            return f"data: {chunk.model_dump_json()}\n\n"

    elif event_type == "message_stop":
        return "data: [DONE]\n\n"

    return None
