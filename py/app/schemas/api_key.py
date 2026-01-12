"""API Key schemas."""

from datetime import datetime

from pydantic import BaseModel, Field


class ApiKeyCreate(BaseModel):
    """Create API Key request."""

    provider_config_id: int = Field(..., description="Associated provider configuration ID")
    name: str = Field(..., min_length=1, max_length=100, description="API Key name")
    expires_at: datetime | None = Field(None, description="Expiration time (UTC)")

    # Usage limits (None means unlimited)
    daily_request_limit: int | None = Field(None, ge=1, description="Daily request limit")
    monthly_request_limit: int | None = Field(None, ge=1, description="Monthly request limit")
    daily_token_limit: int | None = Field(None, ge=1, description="Daily token limit")
    monthly_token_limit: int | None = Field(None, ge=1, description="Monthly token limit")


class ApiKeyUpdate(BaseModel):
    """Update API Key request."""

    name: str | None = Field(None, min_length=1, max_length=100)
    expires_at: datetime | None = None
    is_active: bool | None = None
    daily_request_limit: int | None = Field(None, ge=1)
    monthly_request_limit: int | None = Field(None, ge=1)
    daily_token_limit: int | None = Field(None, ge=1)
    monthly_token_limit: int | None = Field(None, ge=1)


class ApiKeyResponse(BaseModel):
    """API Key response (without full key)."""

    id: int
    name: str
    key_prefix: str  # agw_xxxx...
    provider_config_id: int
    provider: str  # From associated config
    config_name: str  # From associated config

    expires_at: datetime | None
    is_active: bool
    last_used_at: datetime | None

    # Usage limits
    daily_request_limit: int | None
    monthly_request_limit: int | None
    daily_token_limit: int | None
    monthly_token_limit: int | None

    # Current usage
    daily_requests_used: int
    monthly_requests_used: int
    daily_tokens_used: int
    monthly_tokens_used: int

    created_at: datetime

    class Config:
        from_attributes = True


class ApiKeyCreateResponse(BaseModel):
    """API Key creation response (includes full key, only returned once)."""

    id: int
    name: str
    key: str  # Full API Key (agw_xxxxxxxx...)
    key_prefix: str
    provider_config_id: int
    expires_at: datetime | None

    message: str = "Please save this API Key securely. It will only be shown once."


class ApiKeyUsageStats(BaseModel):
    """API Key usage statistics."""

    api_key_id: int
    name: str

    # Today's usage
    today_requests: int
    today_tokens: int

    # This month's usage
    month_requests: int
    month_tokens: int

    # Limits
    daily_request_limit: int | None
    monthly_request_limit: int | None
    daily_token_limit: int | None
    monthly_token_limit: int | None

    # Remaining quota
    daily_requests_remaining: int | None
    monthly_requests_remaining: int | None
    daily_tokens_remaining: int | None
    monthly_tokens_remaining: int | None


class UsageRecordResponse(BaseModel):
    """Usage record response."""

    id: int
    endpoint: str
    model: str
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int
    status_code: int
    created_at: datetime

    class Config:
        from_attributes = True
