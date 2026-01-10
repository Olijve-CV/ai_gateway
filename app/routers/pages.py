"""Page routes for frontend templates."""

from fastapi import APIRouter, Request
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates

templates = Jinja2Templates(directory="app/templates")

router = APIRouter(tags=["Pages"])


@router.get("/login", response_class=HTMLResponse)
async def login_page(request: Request):
    """Render login page."""
    return templates.TemplateResponse("auth/login.html", {"request": request})


@router.get("/register", response_class=HTMLResponse)
async def register_page(request: Request):
    """Render register page."""
    return templates.TemplateResponse("auth/register.html", {"request": request})


@router.get("/dashboard", response_class=HTMLResponse)
async def dashboard_page(request: Request):
    """Render dashboard page."""
    return templates.TemplateResponse("dashboard/index.html", {"request": request})


@router.get("/dashboard/providers", response_class=HTMLResponse)
async def providers_page(request: Request):
    """Render provider configurations page."""
    return templates.TemplateResponse("dashboard/providers.html", {"request": request})


@router.get("/logout")
async def logout():
    """Logout and redirect to login page."""
    return RedirectResponse(url="/login", status_code=302)
