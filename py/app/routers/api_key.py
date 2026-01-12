"""API Key management routes."""

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession

from py.app.database.models import User
from py.app.dependencies.auth import get_current_user
from py.app.dependencies.database import get_db
from py.app.schemas.api_key import (
    ApiKeyCreate,
    ApiKeyCreateResponse,
    ApiKeyResponse,
    ApiKeyUpdate,
    ApiKeyUsageStats,
    UsageRecordResponse,
)
from py.app.services.api_key_service import ApiKeyService

router = APIRouter(prefix="/api/keys", tags=["API Keys"])


@router.post("/", response_model=ApiKeyCreateResponse)
async def create_api_key(
    data: ApiKeyCreate,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """
    Create a new API Key.

    Note: The full API Key will only be returned once. Please save it securely.
    """
    service = ApiKeyService(db)
    try:
        api_key, full_key = await service.create_api_key(current_user, data)
        return ApiKeyCreateResponse(
            id=api_key.id,
            name=api_key.name,
            key=full_key,
            key_prefix=api_key.key_prefix,
            provider_config_id=api_key.provider_config_id,
            expires_at=api_key.expires_at,
        )
    except ValueError as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))


@router.get("/", response_model=list[ApiKeyResponse])
async def list_api_keys(
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Get all API Keys for current user."""
    service = ApiKeyService(db)
    api_keys = await service.get_api_keys(current_user)

    # Build response with provider info
    responses = []
    for key in api_keys:
        await db.refresh(key, ["provider_config"])
        responses.append(
            ApiKeyResponse(
                id=key.id,
                name=key.name,
                key_prefix=key.key_prefix,
                provider_config_id=key.provider_config_id,
                provider=key.provider_config.provider,
                config_name=key.provider_config.name,
                expires_at=key.expires_at,
                is_active=key.is_active,
                last_used_at=key.last_used_at,
                daily_request_limit=key.daily_request_limit,
                monthly_request_limit=key.monthly_request_limit,
                daily_token_limit=key.daily_token_limit,
                monthly_token_limit=key.monthly_token_limit,
                daily_requests_used=key.daily_requests_used,
                monthly_requests_used=key.monthly_requests_used,
                daily_tokens_used=key.daily_tokens_used,
                monthly_tokens_used=key.monthly_tokens_used,
                created_at=key.created_at,
            )
        )
    return responses


@router.get("/{key_id}", response_model=ApiKeyResponse)
async def get_api_key(
    key_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Get details of a specific API Key."""
    service = ApiKeyService(db)
    api_key = await service.get_api_key_by_id(current_user, key_id)
    if not api_key:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="API Key not found",
        )

    await db.refresh(api_key, ["provider_config"])
    return ApiKeyResponse(
        id=api_key.id,
        name=api_key.name,
        key_prefix=api_key.key_prefix,
        provider_config_id=api_key.provider_config_id,
        provider=api_key.provider_config.provider,
        config_name=api_key.provider_config.name,
        expires_at=api_key.expires_at,
        is_active=api_key.is_active,
        last_used_at=api_key.last_used_at,
        daily_request_limit=api_key.daily_request_limit,
        monthly_request_limit=api_key.monthly_request_limit,
        daily_token_limit=api_key.daily_token_limit,
        monthly_token_limit=api_key.monthly_token_limit,
        daily_requests_used=api_key.daily_requests_used,
        monthly_requests_used=api_key.monthly_requests_used,
        daily_tokens_used=api_key.daily_tokens_used,
        monthly_tokens_used=api_key.monthly_tokens_used,
        created_at=api_key.created_at,
    )


@router.put("/{key_id}", response_model=ApiKeyResponse)
async def update_api_key(
    key_id: int,
    data: ApiKeyUpdate,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Update API Key settings."""
    service = ApiKeyService(db)
    api_key = await service.update_api_key(current_user, key_id, data)
    if not api_key:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="API Key not found",
        )

    await db.refresh(api_key, ["provider_config"])
    return ApiKeyResponse(
        id=api_key.id,
        name=api_key.name,
        key_prefix=api_key.key_prefix,
        provider_config_id=api_key.provider_config_id,
        provider=api_key.provider_config.provider,
        config_name=api_key.provider_config.name,
        expires_at=api_key.expires_at,
        is_active=api_key.is_active,
        last_used_at=api_key.last_used_at,
        daily_request_limit=api_key.daily_request_limit,
        monthly_request_limit=api_key.monthly_request_limit,
        daily_token_limit=api_key.daily_token_limit,
        monthly_token_limit=api_key.monthly_token_limit,
        daily_requests_used=api_key.daily_requests_used,
        monthly_requests_used=api_key.monthly_requests_used,
        daily_tokens_used=api_key.daily_tokens_used,
        monthly_tokens_used=api_key.monthly_tokens_used,
        created_at=api_key.created_at,
    )


@router.delete("/{key_id}")
async def delete_api_key(
    key_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Delete an API Key."""
    service = ApiKeyService(db)
    deleted = await service.delete_api_key(current_user, key_id)
    if not deleted:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="API Key not found",
        )
    return {"message": "API Key deleted"}


@router.post("/{key_id}/regenerate", response_model=ApiKeyCreateResponse)
async def regenerate_api_key(
    key_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """
    Regenerate an API Key (old key becomes invalid immediately).

    Note: The new full API Key will only be returned once. Please save it securely.
    """
    service = ApiKeyService(db)
    result = await service.regenerate_api_key(current_user, key_id)
    if not result:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="API Key not found",
        )

    api_key, full_key = result
    return ApiKeyCreateResponse(
        id=api_key.id,
        name=api_key.name,
        key=full_key,
        key_prefix=api_key.key_prefix,
        provider_config_id=api_key.provider_config_id,
        expires_at=api_key.expires_at,
    )


@router.get("/{key_id}/usage", response_model=ApiKeyUsageStats)
async def get_usage_stats(
    key_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Get usage statistics for an API Key."""
    service = ApiKeyService(db)
    stats = await service.get_usage_stats(current_user, key_id)
    if not stats:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="API Key not found",
        )
    return ApiKeyUsageStats(**stats)


@router.get("/{key_id}/history", response_model=list[UsageRecordResponse])
async def get_usage_history(
    key_id: int,
    limit: int = 100,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Get usage history for an API Key."""
    service = ApiKeyService(db)
    api_key = await service.get_api_key_by_id(current_user, key_id)
    if not api_key:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="API Key not found",
        )

    records = await service.get_usage_history(current_user, key_id, limit)
    return records
