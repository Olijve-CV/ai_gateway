"""OpenAI adapter."""

from typing import AsyncIterator

from app.config import settings
from app.utils.http_client import post_request, stream_request


class OpenAIAdapter:
    """Adapter for OpenAI API."""

    def __init__(self, api_key: str, base_url: str | None = None):
        self.api_key = api_key
        self.base_url = base_url or settings.openai_base_url

    def _get_headers(self) -> dict[str, str]:
        return {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }

    async def chat_completions(self, request_data: dict) -> tuple[dict, int]:
        """Send chat completion request. Returns (response_json, status_code)."""
        url = f"{self.base_url}/chat/completions"
        response = await post_request(url, self._get_headers(), request_data)
        return response.json(), response.status_code

    async def chat_completions_stream(
        self, request_data: dict
    ) -> AsyncIterator[bytes]:
        """Send streaming chat completion request."""
        url = f"{self.base_url}/chat/completions"
        request_data["stream"] = True
        async for chunk in stream_request("POST", url, self._get_headers(), request_data):
            yield chunk
