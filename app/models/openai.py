"""OpenAI API models."""

from typing import Any, Literal

from pydantic import BaseModel, Field


# ============ Request Models ============


class FunctionParameters(BaseModel):
    """Function parameters schema."""

    type: str = "object"
    properties: dict[str, Any] = Field(default_factory=dict)
    required: list[str] = Field(default_factory=list)


class FunctionDefinition(BaseModel):
    """Function definition for tool."""

    name: str
    description: str | None = None
    parameters: FunctionParameters | None = None


class Tool(BaseModel):
    """Tool definition."""

    type: Literal["function"] = "function"
    function: FunctionDefinition


class FunctionCall(BaseModel):
    """Function call in message."""

    name: str
    arguments: str  # JSON string


class ToolCall(BaseModel):
    """Tool call in assistant message."""

    id: str
    type: Literal["function"] = "function"
    function: FunctionCall


class Message(BaseModel):
    """Chat message."""

    role: Literal["system", "user", "assistant", "tool"]
    content: str | None = None
    name: str | None = None
    tool_calls: list[ToolCall] | None = None
    tool_call_id: str | None = None  # For tool role


class ChatCompletionRequest(BaseModel):
    """OpenAI chat completion request."""

    model: str
    messages: list[Message]
    temperature: float | None = None
    top_p: float | None = None
    n: int | None = None
    stream: bool = False
    stop: str | list[str] | None = None
    max_tokens: int | None = None
    presence_penalty: float | None = None
    frequency_penalty: float | None = None
    tools: list[Tool] | None = None
    tool_choice: str | dict | None = None
    user: str | None = None


# ============ Response Models ============


class Usage(BaseModel):
    """Token usage."""

    prompt_tokens: int
    completion_tokens: int
    total_tokens: int


class ChoiceMessage(BaseModel):
    """Message in choice."""

    role: Literal["assistant"] = "assistant"
    content: str | None = None
    tool_calls: list[ToolCall] | None = None


class Choice(BaseModel):
    """Completion choice."""

    index: int
    message: ChoiceMessage
    finish_reason: str | None = None


class ChatCompletionResponse(BaseModel):
    """OpenAI chat completion response."""

    id: str
    object: Literal["chat.completion"] = "chat.completion"
    created: int
    model: str
    choices: list[Choice]
    usage: Usage | None = None


# ============ Streaming Models ============


class DeltaMessage(BaseModel):
    """Delta message for streaming."""

    role: str | None = None
    content: str | None = None
    tool_calls: list[ToolCall] | None = None


class StreamChoice(BaseModel):
    """Streaming choice."""

    index: int
    delta: DeltaMessage
    finish_reason: str | None = None


class ChatCompletionChunk(BaseModel):
    """Streaming chunk."""

    id: str
    object: Literal["chat.completion.chunk"] = "chat.completion.chunk"
    created: int
    model: str
    choices: list[StreamChoice]
