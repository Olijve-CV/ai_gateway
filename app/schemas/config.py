"""Configuration management schemas."""

from pydantic import BaseModel, Field


class ProviderConfigCreate(BaseModel):
    """Create provider configuration (endpoint + API key)."""

    provider: str = Field(..., pattern="^(openai|anthropic|gemini)$")
    name: str = Field(..., min_length=1, max_length=100)
    base_url: str = Field(..., min_length=1)
    api_key: str = Field(..., min_length=1)


class ProviderConfigUpdate(BaseModel):
    """Update provider configuration."""

    name: str | None = Field(None, min_length=1, max_length=100)
    base_url: str | None = Field(None, min_length=1)
    api_key: str | None = Field(None, min_length=1)


class ProviderConfigResponse(BaseModel):
    """Provider configuration response."""

    id: int
    provider: str
    name: str
    base_url: str
    key_hint: str
    is_default: bool
    is_active: bool

    class Config:
        from_attributes = True
