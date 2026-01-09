"""AI Gateway - Unified API adapter for OpenAI, Anthropic, and Gemini."""

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from app.config import settings
from app.routers import anthropic, gemini, openai

app = FastAPI(
    title="AI Gateway",
    description="Unified API adapter gateway for OpenAI, Anthropic, and Gemini",
    version="0.1.0",
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


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


# Include routers
app.include_router(openai.router)
app.include_router(anthropic.router)
app.include_router(gemini.router)


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
