"""API Key authentication dependency."""

from fastapi import Depends, HTTPException, Request, status
from fastapi.security import APIKeyHeader, HTTPAuthorizationCredentials, HTTPBearer
from sqlalchemy.ext.asyncio import AsyncSession

from py.app.database.models import ApiKey, User, UserProviderConfig
from py.app.dependencies.database import get_db
from py.app.services.api_key_service import ApiKeyService
from py.app.utils.security import decrypt_api_key

# Support two ways to pass API Key
api_key_header = APIKeyHeader(name="X-API-Key", auto_error=False)
bearer_auth = HTTPBearer(auto_error=False)


class AuthResult:
    """Authentication result containing user and optional provider config."""

    def __init__(
        self,
        user: User,
        api_key: ApiKey | None = None,
        provider_config: UserProviderConfig | None = None,
    ):
        self.user = user
        self.api_key = api_key
        self.provider_config = provider_config

    def get_provider_credentials(self) -> tuple[str, str]:
        """Get provider credentials (base_url, api_key)."""
        if self.provider_config:
            return (
                self.provider_config.base_url,
                decrypt_api_key(self.provider_config.encrypted_key),
            )
        raise ValueError("No provider configuration available")


async def get_auth_from_api_key(
    request: Request,
    x_api_key: str | None = Depends(api_key_header),
    bearer: HTTPAuthorizationCredentials | None = Depends(bearer_auth),
    db: AsyncSession = Depends(get_db),
) -> AuthResult | None:
    """
    Attempt authentication via API Key.

    Supports two methods:
    1. X-API-Key header: agw_xxxxx
    2. Authorization: Bearer agw_xxxxx
    """
    api_key_value = None

    # First check X-API-Key header
    if x_api_key and x_api_key.startswith("agw_"):
        api_key_value = x_api_key
    # Then check Bearer token
    elif bearer and bearer.credentials.startswith("agw_"):
        api_key_value = bearer.credentials

    if not api_key_value:
        return None

    service = ApiKeyService(db)
    result = await service.validate_api_key(api_key_value)

    if not result:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or expired API Key",
        )

    api_key, provider_config = result

    # Get user
    await db.refresh(api_key, ["user"])

    # Store api_key in request.state for later usage recording
    request.state.api_key = api_key

    return AuthResult(
        user=api_key.user,
        api_key=api_key,
        provider_config=provider_config,
    )
