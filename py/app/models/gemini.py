"""Gemini API models."""

from typing import Any, Literal

from pydantic import BaseModel, Field


# ============ Request Models ============


class FunctionDeclaration(BaseModel):
    """Function declaration."""

    name: str
    description: str | None = None
    parameters: dict[str, Any] | None = None


class ToolConfig(BaseModel):
    """Tool configuration."""

    functionDeclarations: list[FunctionDeclaration] = Field(default_factory=list)


class TextPart(BaseModel):
    """Text part."""

    text: str


class FunctionCallPart(BaseModel):
    """Function call part (in model response)."""

    functionCall: dict[str, Any]


class FunctionResponsePart(BaseModel):
    """Function response part (in user message)."""

    functionResponse: dict[str, Any]


Part = TextPart | FunctionCallPart | FunctionResponsePart


class Content(BaseModel):
    """Content with parts."""

    role: Literal["user", "model"]
    parts: list[Part]


class SystemInstruction(BaseModel):
    """System instruction."""

    parts: list[TextPart]


class GenerationConfig(BaseModel):
    """Generation configuration."""

    temperature: float | None = None
    topP: float | None = None
    topK: int | None = None
    maxOutputTokens: int | None = None
    stopSequences: list[str] | None = None


class GenerateContentRequest(BaseModel):
    """Gemini generate content request."""

    contents: list[Content]
    systemInstruction: SystemInstruction | None = None
    generationConfig: GenerationConfig | None = None
    tools: list[ToolConfig] | None = None


# ============ Response Models ============


class UsageMetadata(BaseModel):
    """Usage metadata."""

    promptTokenCount: int | None = None
    candidatesTokenCount: int | None = None
    totalTokenCount: int | None = None


class ResponseContent(BaseModel):
    """Response content."""

    parts: list[Part]
    role: Literal["model"] = "model"


class Candidate(BaseModel):
    """Response candidate."""

    content: ResponseContent
    finishReason: str | None = None
    index: int = 0


class GenerateContentResponse(BaseModel):
    """Gemini generate content response."""

    candidates: list[Candidate]
    usageMetadata: UsageMetadata | None = None


# ============ Streaming uses same models ============
# Gemini streaming returns the same structure incrementally
