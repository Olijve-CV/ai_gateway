"""AI Gateway - Unified API adapter for OpenAI, Anthropic, and Gemini."""

import os
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from fastapi.staticfiles import StaticFiles

from app.config import settings
from app.database import init_db
from app.routers import anthropic, gemini, openai
from app.routers import api_key, auth, config_api, pages


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    # Startup: Create data directory and initialize database
    os.makedirs("data", exist_ok=True)
    await init_db()
    yield
    # Shutdown: cleanup if needed


app = FastAPI(
    title="AI Gateway",
    description="Unified API adapter gateway for OpenAI, Anthropic, and Gemini",
    version="0.1.0",
    lifespan=lifespan,
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Static files
app.mount("/static", StaticFiles(directory="app/static"), name="static")


# Global exception handler
@app.exception_handler(Exception)
async def global_exception_handler(request: Request, exc: Exception):
    """Handle exceptions and return error response."""
    return JSONResponse(
        status_code=500,
        content={
            "error": {
                "message": str(exc),
                "type": type(exc).__name__,
            }
        },
    )


# Include API routers
app.include_router(openai.router)
app.include_router(anthropic.router)
app.include_router(gemini.router)

# Include auth and config routers
app.include_router(auth.router)
app.include_router(config_api.router)
app.include_router(api_key.router)

# Include page routers
app.include_router(pages.router)


@app.get("/")
async def root():
    """Root endpoint."""
    return {
        "name": "AI Gateway",
        "version": "0.1.0",
        "endpoints": {
            "openai": "/v1/chat/completions",
            "anthropic": "/v1/messages",
            "gemini": "/v1/models/{model}:generateContent",
        },
        "dashboard": "/dashboard",
        "docs": "/docs",
    }


@app.get("/health")
async def health():
    """Health check endpoint."""
    return {"status": "healthy"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "app.main:app",
        host=settings.host,
        port=settings.port,
        reload=True,
    )
