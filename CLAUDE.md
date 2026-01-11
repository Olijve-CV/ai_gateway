# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AI Gateway is a unified API adapter gateway for OpenAI, Anthropic, and Gemini AI services. It uses the adapter pattern to route requests and convert between different API formats.

## Development Commands

```bash
# Install dependencies
pip install -r requirements.txt

# Start development server
uvicorn app.main:app --reload

# Alternative startup
python -m app.main
```

Server runs on port 8080 by default.

## Architecture

### Request Flow
```
Client → Router (format endpoint) → Converter → Adapter → Upstream API
Response ← Converter ← Adapter ← Upstream API
```

### Key Components

**Routers** (`app/routers/`): Three format endpoints that accept requests in OpenAI, Anthropic, or Gemini format. Model prefix determines target provider:
- `gpt-*`, `o1-*`, `o3-*` → OpenAI
- `claude-*` → Anthropic
- `gemini-*` → Gemini

**Converters** (`app/converters/`): Six bidirectional converters handle request/response format translation between providers. Each file handles one direction (e.g., `openai_to_anthropic.py`).

**Adapters** (`app/adapters/`): HTTP clients for each provider API. Handle both sync and streaming requests via httpx.

**Models** (`app/models/`): Pydantic v2 models for request/response validation (OpenAI, Anthropic, Gemini formats).

### Authentication System

Two auth modes:
1. **API Key** (`agw_*` prefix): Gateway-issued keys linked to provider configs, supports usage limits
2. **JWT Token**: User session auth for dashboard access

Provider API keys are encrypted and stored in SQLite database (`data/ai_gateway.db`).

### Database Models (`app/database/models.py`)

- `User`: Authentication accounts
- `UserProviderConfig`: Stored provider credentials (encrypted API keys, base URLs)
- `ApiKey`: Gateway-issued API keys with usage limits
- `UsageRecord`: Request/token usage tracking

## Configuration

Settings loaded via pydantic-settings from environment or `.env` file. Key settings in `app/config.py`:
- `openai_base_url`, `anthropic_base_url`, `gemini_base_url`: Provider endpoints
- `jwt_secret`, `encryption_key`: Auto-generated if not set
- `database_url`: SQLite path

## Format Conversion Notes

When modifying converters, be aware of these key differences:
- OpenAI uses `messages[role=system]`, Anthropic uses top-level `system`, Gemini uses `systemInstruction`
- OpenAI tool arguments are JSON strings, Anthropic/Gemini are objects
- Gemini has no tool call IDs, matching is by function name
- Stream format differs: OpenAI/Anthropic use SSE, Gemini uses `alt=sse` URL param
