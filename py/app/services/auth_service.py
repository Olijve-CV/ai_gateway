"""Authentication service."""

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from py.app.database.models import User
from py.app.schemas.auth import RegisterRequest
from py.app.utils.security import create_access_token, hash_password, verify_password


class AuthService:
    """Service for authentication operations."""

    def __init__(self, db: AsyncSession):
        self.db = db

    async def register(self, request: RegisterRequest) -> User:
        """Register a new user."""
        # Check if email already exists
        result = await self.db.execute(select(User).where(User.email == request.email))
        if result.scalar_one_or_none():
            raise ValueError("Email already registered")

        # Check if username already exists
        result = await self.db.execute(select(User).where(User.username == request.username))
        if result.scalar_one_or_none():
            raise ValueError("Username already taken")

        # Create new user
        user = User(
            username=request.username,
            email=request.email,
            hashed_password=hash_password(request.password),
        )
        self.db.add(user)
        await self.db.flush()
        await self.db.refresh(user)
        return user

    async def authenticate(self, email: str, password: str) -> User | None:
        """Authenticate user by email and password."""
        result = await self.db.execute(select(User).where(User.email == email))
        user = result.scalar_one_or_none()

        if user is None:
            return None

        if not verify_password(password, user.hashed_password):
            return None

        if not user.is_active:
            return None

        return user

    def create_token(self, user: User) -> str:
        """Create JWT token for user."""
        return create_access_token(data={"sub": str(user.id)})

    async def get_user_by_id(self, user_id: int) -> User | None:
        """Get user by ID."""
        result = await self.db.execute(select(User).where(User.id == user_id))
        return result.scalar_one_or_none()
