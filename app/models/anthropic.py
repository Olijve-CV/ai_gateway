"""Anthropic API models."""

from typing import Any, Literal

from pydantic import BaseModel, Field


# ============ Request Models ============


class ToolInputSchema(BaseModel):
    """Tool input schema (JSON Schema)."""

    type: str = "object"
    properties: dict[str, Any] = Field(default_factory=dict)
    required: list[str] = Field(default_factory=list)


class ToolDefinition(BaseModel):
    """Tool definition."""

    name: str
    description: str | None = None
    input_schema: ToolInputSchema


class TextContent(BaseModel):
    """Text content block."""

    type: Literal["text"] = "text"
    text: str


class ToolUseContent(BaseModel):
    """Tool use content block (in assistant message)."""

    type: Literal["tool_use"] = "tool_use"
    id: str
    name: str
    input: dict[str, Any]


class ToolResultContent(BaseModel):
    """Tool result content block (in user message)."""

    type: Literal["tool_result"] = "tool_result"
    tool_use_id: str
    content: str | list[TextContent]
    is_error: bool | None = None


ContentBlock = TextContent | ToolUseContent | ToolResultContent


class Message(BaseModel):
    """Chat message."""

    role: Literal["user", "assistant"]
    content: str | list[ContentBlock]


class ToolChoice(BaseModel):
    """Tool choice configuration."""

    type: Literal["auto", "any", "tool"] = "auto"
    name: str | None = None


class MessagesRequest(BaseModel):
    """Anthropic messages request."""

    model: str
    messages: list[Message]
    system: str | None = None
    max_tokens: int
    temperature: float | None = None
    top_p: float | None = None
    top_k: int | None = None
    stop_sequences: list[str] | None = None
    stream: bool = False
    tools: list[ToolDefinition] | None = None
    tool_choice: ToolChoice | None = None


# ============ Response Models ============


class ResponseUsage(BaseModel):
    """Token usage."""

    input_tokens: int
    output_tokens: int


class MessagesResponse(BaseModel):
    """Anthropic messages response."""

    id: str
    type: Literal["message"] = "message"
    role: Literal["assistant"] = "assistant"
    model: str
    content: list[TextContent | ToolUseContent]
    stop_reason: str | None = None
    stop_sequence: str | None = None
    usage: ResponseUsage


# ============ Streaming Models ============


class MessageStartEvent(BaseModel):
    """message_start event."""

    type: Literal["message_start"] = "message_start"
    message: MessagesResponse


class ContentBlockStartEvent(BaseModel):
    """content_block_start event."""

    type: Literal["content_block_start"] = "content_block_start"
    index: int
    content_block: TextContent | ToolUseContent


class ContentBlockDeltaText(BaseModel):
    """Text delta."""

    type: Literal["text_delta"] = "text_delta"
    text: str


class ContentBlockDeltaToolInput(BaseModel):
    """Tool input delta."""

    type: Literal["input_json_delta"] = "input_json_delta"
    partial_json: str


class ContentBlockDeltaEvent(BaseModel):
    """content_block_delta event."""

    type: Literal["content_block_delta"] = "content_block_delta"
    index: int
    delta: ContentBlockDeltaText | ContentBlockDeltaToolInput


class ContentBlockStopEvent(BaseModel):
    """content_block_stop event."""

    type: Literal["content_block_stop"] = "content_block_stop"
    index: int


class MessageDeltaUsage(BaseModel):
    """Usage in message delta."""

    output_tokens: int


class MessageDelta(BaseModel):
    """Message delta."""

    stop_reason: str | None = None
    stop_sequence: str | None = None


class MessageDeltaEvent(BaseModel):
    """message_delta event."""

    type: Literal["message_delta"] = "message_delta"
    delta: MessageDelta
    usage: MessageDeltaUsage


class MessageStopEvent(BaseModel):
    """message_stop event."""

    type: Literal["message_stop"] = "message_stop"
