"""API Key management service."""

import hashlib
import secrets
from datetime import datetime, timezone

from sqlalchemy import and_, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.database.models import ApiKey, User, UserProviderConfig, UsageRecord
from app.schemas.api_key import ApiKeyCreate, ApiKeyUpdate


class ApiKeyService:
    """Service for API Key management."""

    # API Key prefix
    KEY_PREFIX = "agw_"
    # API Key length (excluding prefix)
    KEY_LENGTH = 32

    def __init__(self, db: AsyncSession):
        self.db = db

    # ============ Key Generation & Validation ============

    def generate_api_key(self) -> tuple[str, str, str]:
        """
        Generate a new API Key.
        Returns: (full_key, key_hash, key_prefix)
        """
        # Generate random key
        random_part = secrets.token_urlsafe(self.KEY_LENGTH)
        full_key = f"{self.KEY_PREFIX}{random_part}"

        # Calculate hash (for storage and verification)
        key_hash = hashlib.sha256(full_key.encode()).hexdigest()

        # Generate prefix for display (agw_xxxx...)
        key_prefix = full_key[:8] + "..."

        return full_key, key_hash, key_prefix

    def hash_api_key(self, api_key: str) -> str:
        """Calculate hash of an API Key."""
        return hashlib.sha256(api_key.encode()).hexdigest()

    # ============ CRUD Operations ============

    async def create_api_key(self, user: User, data: ApiKeyCreate) -> tuple[ApiKey, str]:
        """
        Create an API Key.
        Returns: (api_key_model, full_key)
        """
        # Verify provider_config exists and belongs to user
        config = await self._get_user_config(user, data.provider_config_id)
        if not config:
            raise ValueError("Provider configuration not found")

        # Check if this config already has an API Key
        existing = await self.db.execute(
            select(ApiKey).where(ApiKey.provider_config_id == data.provider_config_id)
        )
        if existing.scalar_one_or_none():
            raise ValueError("This provider configuration already has an API Key")

        # Generate API Key
        full_key, key_hash, key_prefix = self.generate_api_key()

        # Create record
        api_key = ApiKey(
            user_id=user.id,
            provider_config_id=data.provider_config_id,
            key_hash=key_hash,
            key_prefix=key_prefix,
            name=data.name,
            expires_at=data.expires_at,
            daily_request_limit=data.daily_request_limit,
            monthly_request_limit=data.monthly_request_limit,
            daily_token_limit=data.daily_token_limit,
            monthly_token_limit=data.monthly_token_limit,
        )

        self.db.add(api_key)
        await self.db.flush()
        await self.db.refresh(api_key)

        return api_key, full_key

    async def get_api_keys(self, user: User) -> list[ApiKey]:
        """Get all API Keys for a user."""
        result = await self.db.execute(
            select(ApiKey).where(ApiKey.user_id == user.id)
        )
        return list(result.scalars().all())

    async def get_api_key_by_id(self, user: User, key_id: int) -> ApiKey | None:
        """Get an API Key by ID."""
        result = await self.db.execute(
            select(ApiKey).where(
                and_(ApiKey.id == key_id, ApiKey.user_id == user.id)
            )
        )
        return result.scalar_one_or_none()

    async def update_api_key(
        self, user: User, key_id: int, data: ApiKeyUpdate
    ) -> ApiKey | None:
        """Update an API Key."""
        api_key = await self.get_api_key_by_id(user, key_id)
        if not api_key:
            return None

        if data.name is not None:
            api_key.name = data.name
        if data.expires_at is not None:
            api_key.expires_at = data.expires_at
        if data.is_active is not None:
            api_key.is_active = data.is_active
        if data.daily_request_limit is not None:
            api_key.daily_request_limit = data.daily_request_limit
        if data.monthly_request_limit is not None:
            api_key.monthly_request_limit = data.monthly_request_limit
        if data.daily_token_limit is not None:
            api_key.daily_token_limit = data.daily_token_limit
        if data.monthly_token_limit is not None:
            api_key.monthly_token_limit = data.monthly_token_limit

        await self.db.flush()
        await self.db.refresh(api_key)
        return api_key

    async def delete_api_key(self, user: User, key_id: int) -> bool:
        """Delete an API Key."""
        api_key = await self.get_api_key_by_id(user, key_id)
        if not api_key:
            return False

        await self.db.delete(api_key)
        return True

    async def regenerate_api_key(self, user: User, key_id: int) -> tuple[ApiKey, str] | None:
        """Regenerate an API Key (keep other settings)."""
        api_key = await self.get_api_key_by_id(user, key_id)
        if not api_key:
            return None

        # Generate new key
        full_key, key_hash, key_prefix = self.generate_api_key()

        api_key.key_hash = key_hash
        api_key.key_prefix = key_prefix

        await self.db.flush()
        await self.db.refresh(api_key)

        return api_key, full_key

    # ============ Authentication & Validation ============

    async def validate_api_key(self, api_key: str) -> tuple[ApiKey, UserProviderConfig] | None:
        """
        Validate an API Key and return associated configuration.
        Returns: (api_key_model, provider_config) or None
        """
        # Calculate hash
        key_hash = self.hash_api_key(api_key)

        # Query
        result = await self.db.execute(
            select(ApiKey).where(ApiKey.key_hash == key_hash)
        )
        api_key_model = result.scalar_one_or_none()

        if not api_key_model:
            return None

        # Check if active
        if not api_key_model.is_active:
            return None

        # Check if expired
        if api_key_model.expires_at:
            if datetime.now(timezone.utc) > api_key_model.expires_at.replace(tzinfo=timezone.utc):
                return None

        # Check and reset usage
        self._check_and_reset_usage(api_key_model)

        if not self._check_usage_limits(api_key_model):
            return None

        # Load associated provider_config
        await self.db.refresh(api_key_model, ["provider_config"])

        if not api_key_model.provider_config or not api_key_model.provider_config.is_active:
            return None

        return api_key_model, api_key_model.provider_config

    def _check_and_reset_usage(self, api_key: ApiKey) -> None:
        """Check and reset expired usage counters."""
        now = datetime.now(timezone.utc)
        daily_reset = api_key.daily_reset_at
        monthly_reset = api_key.monthly_reset_at

        # Add timezone if naive
        if daily_reset.tzinfo is None:
            daily_reset = daily_reset.replace(tzinfo=timezone.utc)
        if monthly_reset.tzinfo is None:
            monthly_reset = monthly_reset.replace(tzinfo=timezone.utc)

        # Check daily reset
        if daily_reset.date() < now.date():
            api_key.daily_requests_used = 0
            api_key.daily_tokens_used = 0
            api_key.daily_reset_at = now

        # Check monthly reset
        if monthly_reset.year < now.year or monthly_reset.month < now.month:
            api_key.monthly_requests_used = 0
            api_key.monthly_tokens_used = 0
            api_key.monthly_reset_at = now

    def _check_usage_limits(self, api_key: ApiKey) -> bool:
        """Check if usage limits are exceeded."""
        # Check daily request limit
        if api_key.daily_request_limit:
            if api_key.daily_requests_used >= api_key.daily_request_limit:
                return False

        # Check monthly request limit
        if api_key.monthly_request_limit:
            if api_key.monthly_requests_used >= api_key.monthly_request_limit:
                return False

        # Check daily token limit
        if api_key.daily_token_limit:
            if api_key.daily_tokens_used >= api_key.daily_token_limit:
                return False

        # Check monthly token limit
        if api_key.monthly_token_limit:
            if api_key.monthly_tokens_used >= api_key.monthly_token_limit:
                return False

        return True

    # ============ Usage Tracking ============

    async def record_usage(
        self,
        api_key: ApiKey,
        endpoint: str,
        model: str,
        prompt_tokens: int,
        completion_tokens: int,
        status_code: int,
    ) -> None:
        """Record API usage."""
        total_tokens = prompt_tokens + completion_tokens

        # Update counters
        api_key.daily_requests_used += 1
        api_key.monthly_requests_used += 1
        api_key.daily_tokens_used += total_tokens
        api_key.monthly_tokens_used += total_tokens
        api_key.last_used_at = datetime.now(timezone.utc)

        # Create detailed record
        record = UsageRecord(
            api_key_id=api_key.id,
            endpoint=endpoint,
            model=model,
            prompt_tokens=prompt_tokens,
            completion_tokens=completion_tokens,
            total_tokens=total_tokens,
            status_code=status_code,
        )
        self.db.add(record)
        await self.db.flush()

    async def get_usage_stats(self, user: User, key_id: int) -> dict | None:
        """Get API Key usage statistics."""
        api_key = await self.get_api_key_by_id(user, key_id)
        if not api_key:
            return None

        self._check_and_reset_usage(api_key)

        return {
            "api_key_id": api_key.id,
            "name": api_key.name,
            "today_requests": api_key.daily_requests_used,
            "today_tokens": api_key.daily_tokens_used,
            "month_requests": api_key.monthly_requests_used,
            "month_tokens": api_key.monthly_tokens_used,
            "daily_request_limit": api_key.daily_request_limit,
            "monthly_request_limit": api_key.monthly_request_limit,
            "daily_token_limit": api_key.daily_token_limit,
            "monthly_token_limit": api_key.monthly_token_limit,
            "daily_requests_remaining": (
                api_key.daily_request_limit - api_key.daily_requests_used
                if api_key.daily_request_limit
                else None
            ),
            "monthly_requests_remaining": (
                api_key.monthly_request_limit - api_key.monthly_requests_used
                if api_key.monthly_request_limit
                else None
            ),
            "daily_tokens_remaining": (
                api_key.daily_token_limit - api_key.daily_tokens_used
                if api_key.daily_token_limit
                else None
            ),
            "monthly_tokens_remaining": (
                api_key.monthly_token_limit - api_key.monthly_tokens_used
                if api_key.monthly_token_limit
                else None
            ),
        }

    async def get_usage_history(
        self, user: User, key_id: int, limit: int = 100
    ) -> list[UsageRecord]:
        """Get usage history records."""
        api_key = await self.get_api_key_by_id(user, key_id)
        if not api_key:
            return []

        result = await self.db.execute(
            select(UsageRecord)
            .where(UsageRecord.api_key_id == key_id)
            .order_by(UsageRecord.created_at.desc())
            .limit(limit)
        )
        return list(result.scalars().all())

    # ============ Helper Methods ============

    async def _get_user_config(
        self, user: User, config_id: int
    ) -> UserProviderConfig | None:
        """Get user's provider configuration."""
        result = await self.db.execute(
            select(UserProviderConfig).where(
                and_(
                    UserProviderConfig.id == config_id,
                    UserProviderConfig.user_id == user.id,
                )
            )
        )
        return result.scalar_one_or_none()
