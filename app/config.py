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

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"


settings = Settings()
