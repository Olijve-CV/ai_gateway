import secrets

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application settings."""

    # Server
    host: str = "0.0.0.0"
    port: int = 8080

    # API endpoints
    openai_base_url: str = "https://api.openai.com/v1"
    anthropic_base_url: str = "https://api.anthropic.com/v1"
    gemini_base_url: str = "https://generativelanguage.googleapis.com/v1beta"

    # Timeouts (seconds)
    request_timeout: float = 300.0

    # Database
    database_url: str = "sqlite+aiosqlite:///./data/ai_gateway.db"

    # JWT Authentication
    jwt_secret: str = secrets.token_urlsafe(32)
    jwt_algorithm: str = "HS256"
    jwt_expire_minutes: int = 60

    # API Key Encryption (32 bytes for AES-256)
    encryption_key: str = secrets.token_urlsafe(32)

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"


settings = Settings()
