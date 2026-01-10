"""Gemini adapter."""

from typing import AsyncIterator

from app.config import settings
from app.utils.http_client import post_request, stream_request


class GeminiAdapter:
    """Adapter for Google Gemini API."""

    def __init__(self, api_key: str, base_url: str | None = None):
        self.api_key = api_key
        self.base_url = base_url or settings.gemini_base_url

    def _get_headers(self) -> dict[str, str]:
        return {
            "Content-Type": "application/json",
        }

    def _get_url(self, model: str, stream: bool = False) -> str:
        action = "streamGenerateContent" if stream else "generateContent"
        url = f"{self.base_url}/models/{model}:{action}?key={self.api_key}"
        if stream:
            url += "&alt=sse"
        return url

    async def generate_content(self, model: str, request_data: dict) -> tuple[dict, int]:
        """Send generate content request. Returns (response_json, status_code)."""
        url = self._get_url(model, stream=False)
        response = await post_request(url, self._get_headers(), request_data)
        return response.json(), response.status_code

    async def generate_content_stream(
        self, model: str, request_data: dict
    ) -> AsyncIterator[bytes]:
        """Send streaming generate content request."""
        url = self._get_url(model, stream=True)
        async for chunk in stream_request("POST", url, self._get_headers(), request_data):
            yield chunk
