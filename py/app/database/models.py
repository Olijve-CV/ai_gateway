"""SQLAlchemy ORM models."""

from datetime import datetime, timezone

from sqlalchemy import Boolean, DateTime, ForeignKey, Index, Integer, String, Text
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column, relationship


class Base(DeclarativeBase):
    """Base class for all models."""

    pass


class User(Base):
    """User model for authentication."""

    __tablename__ = "users"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    username: Mapped[str] = mapped_column(String(50), unique=True, nullable=False, index=True)
    email: Mapped[str] = mapped_column(String(255), unique=True, nullable=False, index=True)
    hashed_password: Mapped[str] = mapped_column(String(255), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True)
    is_admin: Mapped[bool] = mapped_column(Boolean, default=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime, default=lambda: datetime.now(timezone.utc)
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime,
        default=lambda: datetime.now(timezone.utc),
        onupdate=lambda: datetime.now(timezone.utc),
    )

    # Relationships
    provider_configs: Mapped[list["UserProviderConfig"]] = relationship(
        back_populates="user", cascade="all, delete-orphan"
    )
    api_keys: Mapped[list["ApiKey"]] = relationship(
        back_populates="user", cascade="all, delete-orphan"
    )


class UserProviderConfig(Base):
    """User's provider configuration (endpoint + API key combined)."""

    __tablename__ = "user_provider_configs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[int] = mapped_column(Integer, ForeignKey("users.id"), nullable=False)

    # Provider info
    provider: Mapped[str] = mapped_column(String(50), nullable=False)  # openai/anthropic/gemini
    name: Mapped[str] = mapped_column(String(100), nullable=False)  # User-defined config name

    # Endpoint configuration
    base_url: Mapped[str] = mapped_column(String(500), nullable=False)

    # API Key (encrypted)
    encrypted_key: Mapped[str] = mapped_column(Text, nullable=False)
    key_hint: Mapped[str] = mapped_column(String(20), nullable=False)  # e.g., sk-...Hx4f

    # Status
    is_default: Mapped[bool] = mapped_column(Boolean, default=False)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True)

    created_at: Mapped[datetime] = mapped_column(
        DateTime, default=lambda: datetime.now(timezone.utc)
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime,
        default=lambda: datetime.now(timezone.utc),
        onupdate=lambda: datetime.now(timezone.utc),
    )

    # Relationships
    user: Mapped["User"] = relationship(back_populates="provider_configs")
    api_key: Mapped["ApiKey | None"] = relationship(
        back_populates="provider_config", uselist=False
    )

    # Index for faster lookups
    __table_args__ = (
        Index("ix_user_provider_configs", "user_id", "provider"),
    )


class ApiKey(Base):
    """Distributed API Key for accessing provider configurations."""

    __tablename__ = "api_keys"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[int] = mapped_column(Integer, ForeignKey("users.id"), nullable=False)
    provider_config_id: Mapped[int] = mapped_column(
        Integer, ForeignKey("user_provider_configs.id"), nullable=False, unique=True
    )

    # API Key (format: agw_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx)
    key_hash: Mapped[str] = mapped_column(String(255), nullable=False, unique=True, index=True)
    key_prefix: Mapped[str] = mapped_column(String(12), nullable=False)  # agw_xxxx... for display
    name: Mapped[str] = mapped_column(String(100), nullable=False)  # User-defined name

    # Expiration
    expires_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    # Usage limits (None means unlimited)
    daily_request_limit: Mapped[int | None] = mapped_column(Integer, nullable=True)
    monthly_request_limit: Mapped[int | None] = mapped_column(Integer, nullable=True)
    daily_token_limit: Mapped[int | None] = mapped_column(Integer, nullable=True)
    monthly_token_limit: Mapped[int | None] = mapped_column(Integer, nullable=True)

    # Current usage (reset daily/monthly)
    daily_requests_used: Mapped[int] = mapped_column(Integer, default=0)
    monthly_requests_used: Mapped[int] = mapped_column(Integer, default=0)
    daily_tokens_used: Mapped[int] = mapped_column(Integer, default=0)
    monthly_tokens_used: Mapped[int] = mapped_column(Integer, default=0)

    # Reset timestamps
    daily_reset_at: Mapped[datetime] = mapped_column(
        DateTime, default=lambda: datetime.now(timezone.utc)
    )
    monthly_reset_at: Mapped[datetime] = mapped_column(
        DateTime, default=lambda: datetime.now(timezone.utc)
    )

    # Status
    is_active: Mapped[bool] = mapped_column(Boolean, default=True)
    last_used_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    created_at: Mapped[datetime] = mapped_column(
        DateTime, default=lambda: datetime.now(timezone.utc)
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime,
        default=lambda: datetime.now(timezone.utc),
        onupdate=lambda: datetime.now(timezone.utc),
    )

    # Relationships
    user: Mapped["User"] = relationship(back_populates="api_keys")
    provider_config: Mapped["UserProviderConfig"] = relationship(back_populates="api_key")
    usage_records: Mapped[list["UsageRecord"]] = relationship(
        back_populates="api_key", cascade="all, delete-orphan"
    )

    __table_args__ = (
        Index("ix_api_keys_user_id", "user_id"),
        Index("ix_api_keys_expires_at", "expires_at"),
    )


class UsageRecord(Base):
    """Usage record for API Key calls."""

    __tablename__ = "usage_records"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    api_key_id: Mapped[int] = mapped_column(Integer, ForeignKey("api_keys.id"), nullable=False)

    # Request info
    endpoint: Mapped[str] = mapped_column(String(100), nullable=False)  # /v1/chat/completions
    model: Mapped[str] = mapped_column(String(100), nullable=False)

    # Token usage
    prompt_tokens: Mapped[int] = mapped_column(Integer, default=0)
    completion_tokens: Mapped[int] = mapped_column(Integer, default=0)
    total_tokens: Mapped[int] = mapped_column(Integer, default=0)

    # Status
    status_code: Mapped[int] = mapped_column(Integer, nullable=False)

    created_at: Mapped[datetime] = mapped_column(
        DateTime, default=lambda: datetime.now(timezone.utc)
    )

    # Relationships
    api_key: Mapped["ApiKey"] = relationship(back_populates="usage_records")

    __table_args__ = (
        Index("ix_usage_records_api_key_created", "api_key_id", "created_at"),
    )
