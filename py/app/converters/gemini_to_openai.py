"""Convert Gemini format to OpenAI format."""

import json
from typing import Any

from py.app.models import gemini as gem
from py.app.models import openai as oai


def convert_request(req: gem.GenerateContentRequest) -> oai.ChatCompletionRequest:
    """Convert Gemini generate content request to OpenAI chat completion request."""
    messages: list[oai.Message] = []

    # Add system message if present
    if req.systemInstruction:
        system_text = "".join(p.text for p in req.systemInstruction.parts)
        messages.append(oai.Message(role="system", content=system_text))

    # Convert contents
    for content in req.contents:
        role = "user" if content.role == "user" else "assistant"

        text_parts = []
        tool_calls = []
        function_responses = []

        for part in content.parts:
            if isinstance(part, gem.TextPart):
                text_parts.append(part.text)
            elif isinstance(part, gem.FunctionCallPart):
                fc = part.functionCall
                tool_calls.append(
                    oai.ToolCall(
                        id=f"call_{len(tool_calls)}",
                        type="function",
                        function=oai.FunctionCall(
                            name=fc.get("name", ""),
                            arguments=json.dumps(fc.get("args", {})),
                        ),
                    )
                )
            elif isinstance(part, gem.FunctionResponsePart):
                fr = part.functionResponse
                function_responses.append(
                    {
                        "name": fr.get("name", ""),
                        "response": fr.get("response", {}),
                    }
                )

        if role == "user":
            if function_responses:
                for fr in function_responses:
                    messages.append(
                        oai.Message(
                            role="tool",
                            name=fr["name"],
                            content=json.dumps(fr["response"]),
                            tool_call_id=f"call_{fr['name']}",
                        )
                    )
            elif text_parts:
                messages.append(oai.Message(role="user", content="".join(text_parts)))
        else:  # assistant
            messages.append(
                oai.Message(
                    role="assistant",
                    content="".join(text_parts) if text_parts else None,
                    tool_calls=tool_calls if tool_calls else None,
                )
            )

    # Convert tools
    tools = None
    if req.tools:
        tools = []
        for tool_config in req.tools:
            for fd in tool_config.functionDeclarations:
                tools.append(
                    oai.Tool(
                        type="function",
                        function=oai.FunctionDefinition(
                            name=fd.name,
                            description=fd.description,
                            parameters=oai.FunctionParameters(
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
                        ),
                    )
                )

    # Generation config
    gen_config = req.generationConfig
    return oai.ChatCompletionRequest(
        model="",  # Will be set by caller
        messages=messages,
        temperature=gen_config.temperature if gen_config else None,
        top_p=gen_config.topP if gen_config else None,
        max_tokens=gen_config.maxOutputTokens if gen_config else None,
        stop=gen_config.stopSequences if gen_config else None,
        tools=tools,
    )


def convert_response(
    resp: oai.ChatCompletionResponse, model: str
) -> gem.GenerateContentResponse:
    """Convert OpenAI chat completion response to Gemini generate content response."""
    parts: list[gem.Part] = []

    if resp.choices:
        choice = resp.choices[0]
        msg = choice.message

        if msg.content:
            parts.append(gem.TextPart(text=msg.content))

        if msg.tool_calls:
            for tc in msg.tool_calls:
                parts.append(
                    gem.FunctionCallPart(
                        functionCall={
                            "name": tc.function.name,
                            "args": json.loads(tc.function.arguments),
                        }
                    )
                )

    # Map finish reason
    finish_reason = resp.choices[0].finish_reason if resp.choices else "stop"
    finish_reason_map = {
        "stop": "STOP",
        "length": "MAX_TOKENS",
        "tool_calls": "STOP",
        "content_filter": "SAFETY",
    }
    gemini_finish = finish_reason_map.get(finish_reason or "stop", "STOP")

    return gem.GenerateContentResponse(
        candidates=[
            gem.Candidate(
                content=gem.ResponseContent(parts=parts if parts else [gem.TextPart(text="")]),
                finishReason=gemini_finish,
                index=0,
            )
        ],
        usageMetadata=gem.UsageMetadata(
            promptTokenCount=resp.usage.prompt_tokens if resp.usage else 0,
            candidatesTokenCount=resp.usage.completion_tokens if resp.usage else 0,
            totalTokenCount=resp.usage.total_tokens if resp.usage else 0,
        ),
    )


def convert_stream_chunk(data: dict[str, Any]) -> dict[str, Any] | None:
    """Convert OpenAI stream chunk to Gemini stream chunk.

    Returns dict or None if chunk should be skipped.
    """
    choices = data.get("choices", [])
    if not choices:
        return None

    choice = choices[0]
    delta = choice.get("delta", {})

    parts = []
    content = delta.get("content")
    if content:
        parts.append({"text": content})

    if not parts:
        finish_reason = choice.get("finish_reason")
        if finish_reason:
            finish_reason_map = {
                "stop": "STOP",
                "length": "MAX_TOKENS",
                "content_filter": "SAFETY",
            }
            return {
                "candidates": [
                    {
                        "content": {"parts": [], "role": "model"},
                        "finishReason": finish_reason_map.get(finish_reason, "STOP"),
                        "index": 0,
                    }
                ]
            }
        return None

    return {
        "candidates": [
            {
                "content": {"parts": parts, "role": "model"},
                "index": 0,
            }
        ]
    }
