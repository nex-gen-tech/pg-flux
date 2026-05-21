"""
Pydantic models mirroring the pg-flux schema.

Kept hand-written here because pg-flux's codegen ships Go + TypeScript
out of the box; Python types live alongside until a Python emitter
lands. Treat these as the contract with ../schema/*.sql — when you
change a column there, change it here.
"""
from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Literal, Optional

from pydantic import BaseModel, EmailStr, Field, constr


# Matches `CREATE TYPE public.todo_priority AS ENUM (...)` in schema/types.sql.
class Priority(str, Enum):
    low = "low"
    normal = "normal"
    high = "high"
    urgent = "urgent"


class UserCreate(BaseModel):
    email: EmailStr
    handle: constr(min_length=2, max_length=64)


class User(BaseModel):
    id: int
    email: str
    handle: str
    email_verified: bool
    created_at: datetime


class TodoCreate(BaseModel):
    title: constr(min_length=1, max_length=200)
    body: str = ""
    priority: Priority = Priority.normal
    category_id: Optional[int] = None


class TodoUpdate(BaseModel):
    title: Optional[constr(min_length=1, max_length=200)] = None
    body: Optional[str] = None
    priority: Optional[Priority] = None
    done: Optional[bool] = None
    category_id: Optional[int] = None


class Todo(BaseModel):
    id: int
    user_id: int
    category_id: Optional[int]
    title: str
    title_lower: str
    body: str
    priority: Priority
    done: bool
    deleted_at: Optional[datetime]
    created_at: datetime
    updated_at: datetime


class Category(BaseModel):
    id: int
    name: str
    color: str
