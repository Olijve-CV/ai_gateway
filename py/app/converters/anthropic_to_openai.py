"""Convert Anthropic format to OpenAI format."""

import json
from typing import Any

from py.app.models import anthropic as ant
from py.app.models import openai as oai


def convert_request(req: ant.MessagesRequest) -> oai.ChatCompletionRequest:
    """Convert Anthropic messages request to OpenAI chat completion request."""
    messages: list[oai.Message] = []

    # Add system message if present
    if req.system:
        messages.append(oai.Message(role="system", content=req.system))

    # Convert messages
    for msg in req.messages:
        if msg.role == "user":
            if isinstance(msg.content, str):
                messages.append(oai.Message(role="user", content=msg.content))
            else:
                # Handle content blocks
                for block in msg.content:
                    if isinstance(block, ant.TextContent):
                        messages.append(oai.Message(role="user", content=block.text))
                    elif isinstance(block, ant.ToolResultContent):
                        content = (
                            block.content
                            if isinstance(block.content, str)
                            else block.content[0].text
                        )
                        messages.append(
                            oai.Message(
                                role="tool",
                                content=content,
                                tool_call_id=block.tool_use_id,
                            )
                        )
        elif msg.role == "assistant":
            if isinstance(msg.content, str):
                messages.append(oai.Message(role="assistant", content=msg.content))
            else:
                # Handle content blocks with potential tool_use
                text_content = ""
                tool_calls = []
                for block in msg.content:
                    if isinstance(block, ant.TextContent):
                        text_content += block.text
                    elif isinstance(block, ant.ToolUseContent):
                        tool_calls.append(
                            oai.ToolCall(
                                id=block.id,
                                type="function",
                                function=oai.FunctionCall(
                                    name=block.name,
                                    arguments=json.dumps(block.input),
                                ),
                            )
                        )
                messages.append(
                    oai.Message(
                        role="assistant",
                        content=text_content if text_content else None,
                        tool_calls=tool_calls if tool_calls else None,
                    )
                )

    # Convert tools
    tools = None
    if req.tools:
        tools = []
        for tool in req.tools:
            tools.append(
                oai.Tool(
                    type="function",
                    function=oai.FunctionDefinition(
                        name=tool.name,
                        description=tool.description,
                        parameters=oai.FunctionParameters(
                            type=tool.input_schema.type,
                            properties=tool.input_schema.properties,
                            required=tool.input_schema.required,
                        ),
                    ),
                )
            )

    # Convert tool_choice
    tool_choice = None
    if req.tool_choice:
        if req.tool_choice.type == "auto":
            tool_choice = "auto"
        elif req.tool_choice.type == "any":
            tool_choice = "required"
        elif req.tool_choice.type == "tool":
            tool_choice = {"type": "function", "function": {"name": req.tool_choice.name}}

    return oai.ChatCompletionRequest(
        model=req.model,
        messages=messages,
        max_tokens=req.max_tokens,
        temperature=req.temperature,
        top_p=req.top_p,
        stop=req.stop_sequences,
        stream=req.stream,
        tools=tools,
        tool_choice=tool_choice,
    )


def convert_response(resp: oai.ChatCompletionResponse) -> ant.MessagesResponse:
    """Convert OpenAI chat completion response to Anthropic messages response."""
    content: list[ant.TextContent | ant.ToolUseContent] = []

    if resp.choices:
        choice = resp.choices[0]
        msg = choice.message

        if msg.content:
            content.append(ant.TextContent(text=msg.content))

        if msg.tool_calls:
            for tc in msg.tool_calls:
                content.append(
                    ant.ToolUseContent(
                        id=tc.id,
                        name=tc.function.name,
                        input=json.loads(tc.function.arguments),
                    )
                )

    # Map finish reason
    finish_reason = resp.choices[0].finish_reason if resp.choices else "end_turn"
    stop_reason_map = {
        "stop": "end_turn",
        "length": "max_tokens",
        "tool_calls": "tool_use",
        "content_filter": "end_turn",
    }
    stop_reason = stop_reason_map.get(finish_reason or "stop", "end_turn")

    return ant.MessagesResponse(
        id=resp.id,
        model=resp.model,
        content=content if content else [ant.TextContent(text="")],
        stop_reason=stop_reason,
        usage=ant.ResponseUsage(
            input_tokens=resp.usage.prompt_tokens if resp.usage else 0,
            output_tokens=resp.usage.completion_tokens if resp.usage else 0,
        ),
    )


def convert_stream_chunk(data: dict[str, Any]) -> str | None:
    """Convert OpenAI stream chunk to Anthropic stream event.

    Returns SSE formatted string or None if chunk should be skipped.
    """
    choices = data.get("choices", [])
    if not choices:
        return None

    choice = choices[0]
    delta = choice.get("delta", {})
    finish_reason = choice.get("finish_reason")

    # Role delta (first chunk)
    if delta.get("role") == "assistant":
        event = {
            "type": "message_start",
            "message": {
                "id": data.get("id", ""),
                "type": "message",
                "role": "assistant",
                "model": data.get("model", ""),
                "content": [],
                "stop_reason": None,
                "usage": {"input_tokens": 0, "output_tokens": 0},
            },
        }
        return f"event: message_start\ndata: {json.dumps(event)}\n\n"

    # Content delta
    content = delta.get("content")
    if content:
        event = {
            "type": "content_block_delta",
            "index": 0,
            "delta": {"type": "text_delta", "text": content},
        }
        return f"event: content_block_delta\ndata: {json.dumps(event)}\n\n"

    # Finish
    if finish_reason:
        stop_reason_map = {
            "stop": "end_turn",
            "length": "max_tokens",
            "tool_calls": "tool_use",
        }
        stop_reason = stop_reason_map.get(finish_reason, "end_turn")
        event = {
            "type": "message_delta",
            "delta": {"stop_reason": stop_reason},
            "usage": {"output_tokens": 0},
        }
        result = f"event: message_delta\ndata: {json.dumps(event)}\n\n"
        result += "event: message_stop\ndata: {\"type\": \"message_stop\"}\n\n"
        return result

    return None
