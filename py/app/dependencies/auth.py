"""Authentication dependencies."""

from fastapi import Depends, HTTPException, Request, status
from fastapi.security import APIKeyHeader, HTTPAuthorizationCredentials, HTTPBearer
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from py.app.database.models import ApiKey, User, UserProviderConfig
from py.app.dependencies.database import get_db
from py.app.utils.security import decode_access_token, decrypt_api_key

security = HTTPBearer(auto_error=False)
api_key_header = APIKeyHeader(name="X-API-Key", auto_error=False)


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


async def get_current_user(
    credentials: HTTPAuthorizationCredentials | None = Depends(security),
    db: AsyncSession = Depends(get_db),
) -> User:
    """Get current authenticated user from JWT token."""
    if credentials is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Not authenticated",
            headers={"WWW-Authenticate": "Bearer"},
        )

    token = credentials.credentials
    payload = decode_access_token(token)

    if payload is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or expired token",
            headers={"WWW-Authenticate": "Bearer"},
        )

    user_id = payload.get("sub")
    if user_id is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid token payload",
        )

    result = await db.execute(select(User).where(User.id == int(user_id)))
    user = result.scalar_one_or_none()

    if user is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="User not found",
        )

    if not user.is_active:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="User is inactive",
        )

    return user


async def get_optional_user(
    credentials: HTTPAuthorizationCredentials | None = Depends(security),
    db: AsyncSession = Depends(get_db),
) -> User | None:
    """Get current user if authenticated, otherwise return None."""
    if credentials is None:
        return None

    try:
        return await get_current_user(credentials, db)
    except HTTPException:
        return None


async def get_auth_context(
    request: Request,
    x_api_key: str | None = Depends(api_key_header),
    credentials: HTTPAuthorizationCredentials | None = Depends(security),
    db: AsyncSession = Depends(get_db),
) -> AuthResult:
    """
    Unified authentication context supporting both JWT and API Key.

    Authentication order:
    1. If token starts with "agw_", try API Key authentication
    2. Otherwise try JWT authentication

    API Key authentication automatically associates provider configuration.
    """
    from py.app.services.api_key_service import ApiKeyService

    api_key_value = None

    # Check X-API-Key header first
    if x_api_key and x_api_key.startswith("agw_"):
        api_key_value = x_api_key
    # Then check Bearer token
    elif credentials and credentials.credentials.startswith("agw_"):
        api_key_value = credentials.credentials

    # Try API Key authentication
    if api_key_value:
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

    # Fall back to JWT authentication
    if credentials is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Not authenticated",
            headers={"WWW-Authenticate": "Bearer"},
        )

    token = credentials.credentials

    # JWT authentication
    payload = decode_access_token(token)
    if payload is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or expired token",
            headers={"WWW-Authenticate": "Bearer"},
        )

    user_id = payload.get("sub")
    if user_id is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid token payload",
        )

    result = await db.execute(select(User).where(User.id == int(user_id)))
    user = result.scalar_one_or_none()

    if user is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="User not found",
        )

    if not user.is_active:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="User is inactive",
        )

    return AuthResult(user=user)
