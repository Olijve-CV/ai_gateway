"""Security utilities for password hashing, JWT, and API key encryption."""

import base64
import os
from datetime import datetime, timedelta, timezone
from typing import Any

import bcrypt
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from jose import JWTError, jwt

from py.app.config import settings


# Password hashing (using bcrypt directly)
def hash_password(password: str) -> str:
    """Hash a password using bcrypt."""
    # Truncate to 72 bytes (bcrypt limit)
    password_bytes = password.encode("utf-8")[:72]
    salt = bcrypt.gensalt()
    hashed = bcrypt.hashpw(password_bytes, salt)
    return hashed.decode("utf-8")


def verify_password(plain_password: str, hashed_password: str) -> bool:
    """Verify a password against its hash."""
    # Truncate to 72 bytes (bcrypt limit)
    password_bytes = plain_password.encode("utf-8")[:72]
    hashed_bytes = hashed_password.encode("utf-8")
    try:
        return bcrypt.checkpw(password_bytes, hashed_bytes)
    except Exception:
        return False


# JWT Token
def create_access_token(data: dict[str, Any], expires_delta: timedelta | None = None) -> str:
    """Create a JWT access token."""
    to_encode = data.copy()
    if expires_delta:
        expire = datetime.now(timezone.utc) + expires_delta
    else:
        expire = datetime.now(timezone.utc) + timedelta(minutes=settings.jwt_expire_minutes)
    to_encode.update({"exp": expire})
    encoded_jwt = jwt.encode(to_encode, settings.jwt_secret, algorithm=settings.jwt_algorithm)
    return encoded_jwt


def decode_access_token(token: str) -> dict[str, Any] | None:
    """Decode and verify a JWT access token."""
    try:
        payload = jwt.decode(token, settings.jwt_secret, algorithms=[settings.jwt_algorithm])
        return payload
    except JWTError:
        return None


# API Key Encryption (AES-256-GCM)
def _get_encryption_key() -> bytes:
    """Get the 32-byte encryption key from settings."""
    key = settings.encryption_key
    # Ensure key is 32 bytes (256 bits) for AES-256
    key_bytes = key.encode()[:32].ljust(32, b"\0")
    return key_bytes


def encrypt_api_key(api_key: str) -> str:
    """Encrypt an API key using AES-256-GCM."""
    key = _get_encryption_key()
    aesgcm = AESGCM(key)
    nonce = os.urandom(12)  # 96-bit nonce for GCM
    ciphertext = aesgcm.encrypt(nonce, api_key.encode(), None)
    # Combine nonce + ciphertext and base64 encode
    return base64.b64encode(nonce + ciphertext).decode()


def decrypt_api_key(encrypted_key: str) -> str:
    """Decrypt an API key."""
    key = _get_encryption_key()
    aesgcm = AESGCM(key)
    data = base64.b64decode(encrypted_key)
    nonce, ciphertext = data[:12], data[12:]
    return aesgcm.decrypt(nonce, ciphertext, None).decode()


def get_api_key_hint(api_key: str) -> str:
    """Get a hint for the API key (first 4 and last 4 characters)."""
    if len(api_key) <= 12:
        return api_key[:2] + "..." + api_key[-2:]
    return api_key[:4] + "..." + api_key[-4:]
