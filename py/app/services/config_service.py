"""Configuration management service."""

from sqlalchemy import and_, select
from sqlalchemy.ext.asyncio import AsyncSession

from py.app.database.models import User, UserProviderConfig
from py.app.schemas.config import ProviderConfigCreate, ProviderConfigUpdate
from py.app.utils.security import decrypt_api_key, encrypt_api_key, get_api_key_hint


class ConfigService:
    """Service for user provider configuration management."""

    def __init__(self, db: AsyncSession):
        self.db = db

    async def get_configs(self, user: User) -> list[UserProviderConfig]:
        """Get all provider configurations for a user."""
        result = await self.db.execute(
            select(UserProviderConfig).where(UserProviderConfig.user_id == user.id)
        )
        return list(result.scalars().all())

    async def get_configs_by_provider(self, user: User, provider: str) -> list[UserProviderConfig]:
        """Get all configurations for a specific provider."""
        result = await self.db.execute(
            select(UserProviderConfig).where(
                and_(
                    UserProviderConfig.user_id == user.id,
                    UserProviderConfig.provider == provider,
                )
            )
        )
        return list(result.scalars().all())

    async def get_config_by_id(self, user: User, config_id: int) -> UserProviderConfig | None:
        """Get a specific configuration by ID."""
        result = await self.db.execute(
            select(UserProviderConfig).where(
                and_(
                    UserProviderConfig.user_id == user.id,
                    UserProviderConfig.id == config_id,
                )
            )
        )
        return result.scalar_one_or_none()

    async def create_config(self, user: User, data: ProviderConfigCreate) -> UserProviderConfig:
        """Create a new provider configuration."""
        # Check if this is the first config for this provider
        existing = await self.get_configs_by_provider(user, data.provider)
        is_first = len(existing) == 0

        config = UserProviderConfig(
            user_id=user.id,
            provider=data.provider,
            name=data.name,
            base_url=data.base_url,
            encrypted_key=encrypt_api_key(data.api_key),
            key_hint=get_api_key_hint(data.api_key),
            is_default=is_first,  # First config is default
        )
        self.db.add(config)
        await self.db.flush()
        await self.db.refresh(config)
        return config

    async def update_config(
        self, user: User, config_id: int, data: ProviderConfigUpdate
    ) -> UserProviderConfig | None:
        """Update a provider configuration."""
        config = await self.get_config_by_id(user, config_id)
        if not config:
            return None

        if data.name is not None:
            config.name = data.name

        if data.base_url is not None:
            config.base_url = data.base_url

        if data.api_key is not None:
            config.encrypted_key = encrypt_api_key(data.api_key)
            config.key_hint = get_api_key_hint(data.api_key)

        await self.db.flush()
        await self.db.refresh(config)
        return config

    async def delete_config(self, user: User, config_id: int) -> bool:
        """Delete a provider configuration."""
        config = await self.get_config_by_id(user, config_id)
        if not config:
            return False

        was_default = config.is_default
        provider = config.provider

        await self.db.delete(config)

        # If deleted config was default, set another as default
        if was_default:
            remaining = await self.get_configs_by_provider(user, provider)
            if remaining:
                remaining[0].is_default = True

        return True

    async def set_default(self, user: User, config_id: int) -> UserProviderConfig | None:
        """Set a configuration as default for its provider."""
        config = await self.get_config_by_id(user, config_id)
        if not config:
            return None

        # Unset other defaults for this provider
        result = await self.db.execute(
            select(UserProviderConfig).where(
                and_(
                    UserProviderConfig.user_id == user.id,
                    UserProviderConfig.provider == config.provider,
                    UserProviderConfig.is_default == True,
                )
            )
        )
        for c in result.scalars().all():
            c.is_default = False

        # Set this config as default
        config.is_default = True
        await self.db.flush()
        await self.db.refresh(config)
        return config

    async def toggle_active(self, user: User, config_id: int) -> UserProviderConfig | None:
        """Toggle configuration active status."""
        config = await self.get_config_by_id(user, config_id)
        if config:
            config.is_active = not config.is_active
            await self.db.flush()
            await self.db.refresh(config)
        return config

    async def get_default_config(self, user: User, provider: str) -> tuple[str, str] | None:
        """Get the default configuration for a provider (base_url, decrypted_key)."""
        result = await self.db.execute(
            select(UserProviderConfig).where(
                and_(
                    UserProviderConfig.user_id == user.id,
                    UserProviderConfig.provider == provider,
                    UserProviderConfig.is_default == True,
                    UserProviderConfig.is_active == True,
                )
            )
        )
        config = result.scalar_one_or_none()
        if config:
            return config.base_url, decrypt_api_key(config.encrypted_key)
        return None
