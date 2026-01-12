"""Configuration management API routes."""

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession

from py.app.database.models import User
from py.app.dependencies.auth import get_current_user
from py.app.dependencies.database import get_db
from py.app.schemas.config import (
    ProviderConfigCreate,
    ProviderConfigResponse,
    ProviderConfigUpdate,
)
from py.app.services.config_service import ConfigService

router = APIRouter(prefix="/api/config", tags=["Configuration"])


@router.get("/providers", response_model=list[ProviderConfigResponse])
async def get_all_configs(
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Get all provider configurations for current user."""
    service = ConfigService(db)
    return await service.get_configs(current_user)


@router.get("/providers/{provider}", response_model=list[ProviderConfigResponse])
async def get_configs_by_provider(
    provider: str,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Get all configurations for a specific provider."""
    service = ConfigService(db)
    return await service.get_configs_by_provider(current_user, provider)


@router.post("/providers", response_model=ProviderConfigResponse)
async def create_config(
    data: ProviderConfigCreate,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Create a new provider configuration."""
    service = ConfigService(db)
    return await service.create_config(current_user, data)


@router.get("/providers/id/{config_id}", response_model=ProviderConfigResponse)
async def get_config(
    config_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Get a specific configuration by ID."""
    service = ConfigService(db)
    config = await service.get_config_by_id(current_user, config_id)
    if not config:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Configuration not found",
        )
    return config


@router.put("/providers/{config_id}", response_model=ProviderConfigResponse)
async def update_config(
    config_id: int,
    data: ProviderConfigUpdate,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Update a provider configuration."""
    service = ConfigService(db)
    config = await service.update_config(current_user, config_id, data)
    if not config:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Configuration not found",
        )
    return config


@router.delete("/providers/{config_id}")
async def delete_config(
    config_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Delete a provider configuration."""
    service = ConfigService(db)
    deleted = await service.delete_config(current_user, config_id)
    if not deleted:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Configuration not found",
        )
    return {"message": "Configuration deleted"}


@router.put("/providers/{config_id}/default", response_model=ProviderConfigResponse)
async def set_default_config(
    config_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Set a configuration as default for its provider."""
    service = ConfigService(db)
    config = await service.set_default(current_user, config_id)
    if not config:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Configuration not found",
        )
    return config


@router.put("/providers/{config_id}/toggle", response_model=ProviderConfigResponse)
async def toggle_config_active(
    config_id: int,
    current_user: User = Depends(get_current_user),
    db: AsyncSession = Depends(get_db),
):
    """Toggle configuration active status."""
    service = ConfigService(db)
    config = await service.toggle_active(current_user, config_id)
    if not config:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="Configuration not found",
        )
    return config
