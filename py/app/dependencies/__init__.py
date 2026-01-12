"""Dependencies module."""

from py.app.dependencies.auth import get_current_user
from py.app.dependencies.database import get_db

__all__ = ["get_db", "get_current_user"]
