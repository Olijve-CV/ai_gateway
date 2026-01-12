"""HTTP client utilities."""

from typing import AsyncIterator

import httpx

from py.app.config import settings


async def stream_request(
    method: str,
    url: str,
    headers: dict[str, str],
    json_data: dict | None = None,
) -> AsyncIterator[bytes]:
    """Make a streaming HTTP request and yield chunks."""
    async with httpx.AsyncClient(timeout=settings.request_timeout) as client:
        async with client.stream(
            method,
            url,
            headers=headers,
            json=json_data,
        ) as response:
            response.raise_for_status()
            async for chunk in response.aiter_bytes():
                yield chunk


async def post_request(
    url: str,
    headers: dict[str, str],
    json_data: dict | None = None,
) -> httpx.Response:
    """Make a POST request."""
    async with httpx.AsyncClient(timeout=settings.request_timeout) as client:
        response = await client.post(url, headers=headers, json=json_data)
        return response
