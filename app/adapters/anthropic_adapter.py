"""Anthropic adapter."""

from typing import AsyncIterator

from app.config import settings
from app.utils.http_client import post_request, stream_request


class AnthropicAdapter:
    """Adapter for Anthropic API."""

    def __init__(self, api_key: str, base_url: str | None = None):
        self.api_key = api_key
        self.base_url = base_url or settings.anthropic_base_url

    def _get_headers(self) -> dict[str, str]:
        return {
            "x-api-key": self.api_key,
            "Content-Type": "application/json",
            "anthropic-version": "2023-06-01",
        }

    async def messages(self, request_data: dict) -> tuple[dict, int]:
        """Send messages request. Returns (response_json, status_code)."""
        url = f"{self.base_url}/messages"
        response = await post_request(url, self._get_headers(), request_data)
        return response.json(), response.status_code

    async def messages_stream(self, request_data: dict) -> AsyncIterator[bytes]:
        """Send streaming messages request."""
        url = f"{self.base_url}/messages"
        request_data["stream"] = True
        async for chunk in stream_request("POST", url, self._get_headers(), request_data):
            yield chunk
