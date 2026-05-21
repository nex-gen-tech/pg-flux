"""
FastAPI service backed by a pg-flux-managed schema.

This file is the API surface — routes, query plumbing, error mapping.
The schema itself lives in ../schema/*.sql and is applied through
`pg-flux migrate apply` (see README.md).
"""
from __future__ import annotations

from typing import List

from dotenv import load_dotenv
from fastapi import FastAPI, HTTPException
from psycopg import errors as pg_errors

load_dotenv()  # picks up DATABASE_URL from .env

from .db import conn  # noqa: E402  (load_dotenv must run before db import)
from .models import (
    Category,
    Priority,
    Todo,
    TodoCreate,
    TodoUpdate,
    User,
    UserCreate,
)

app = FastAPI(title="pg-flux FastAPI example — todos")


# ---------- users ------------------------------------------------------------

@app.post("/users", response_model=User, status_code=201)
def create_user(payload: UserCreate) -> User:
    with conn() as c, c.cursor() as cur:
        try:
            cur.execute(
                """
                INSERT INTO public.users (email, handle)
                VALUES (%s, %s)
                RETURNING id, email, handle, email_verified, created_at
                """,
                (payload.email, payload.handle),
            )
        except pg_errors.UniqueViolation as e:
            raise HTTPException(status_code=409, detail=str(e.diag.constraint_name))
        row = cur.fetchone()
        return User(**row)


@app.get("/users/{user_id}", response_model=User)
def get_user(user_id: int) -> User:
    with conn() as c, c.cursor() as cur:
        cur.execute(
            "SELECT id, email, handle, email_verified, created_at FROM public.users WHERE id = %s",
            (user_id,),
        )
        row = cur.fetchone()
        if not row:
            raise HTTPException(status_code=404, detail="user not found")
        return User(**row)


# ---------- todos ------------------------------------------------------------

@app.post("/users/{user_id}/todos", response_model=Todo, status_code=201)
def create_todo(user_id: int, payload: TodoCreate) -> Todo:
    with conn() as c, c.cursor() as cur:
        cur.execute(
            """
            INSERT INTO public.todos (user_id, category_id, title, body, priority)
            VALUES (%s, %s, %s, %s, %s)
            RETURNING id, user_id, category_id, title, title_lower, body, priority, done,
                      deleted_at, created_at, updated_at
            """,
            (user_id, payload.category_id, payload.title, payload.body, payload.priority.value),
        )
        return Todo(**cur.fetchone())


@app.get("/users/{user_id}/todos", response_model=List[Todo])
def list_todos(user_id: int, done: bool | None = None) -> List[Todo]:
    with conn() as c, c.cursor() as cur:
        if done is None:
            cur.execute(
                """
                SELECT id, user_id, category_id, title, title_lower, body, priority, done,
                       deleted_at, created_at, updated_at
                FROM public.todos
                WHERE user_id = %s AND deleted_at IS NULL
                ORDER BY created_at DESC
                """,
                (user_id,),
            )
        else:
            cur.execute(
                """
                SELECT id, user_id, category_id, title, title_lower, body, priority, done,
                       deleted_at, created_at, updated_at
                FROM public.todos
                WHERE user_id = %s AND deleted_at IS NULL AND done = %s
                ORDER BY created_at DESC
                """,
                (user_id, done),
            )
        return [Todo(**r) for r in cur.fetchall()]


@app.patch("/todos/{todo_id}", response_model=Todo)
def update_todo(todo_id: int, payload: TodoUpdate) -> Todo:
    fields = payload.model_dump(exclude_unset=True)
    if not fields:
        raise HTTPException(status_code=400, detail="no fields to update")

    set_parts = []
    values = []
    for k, v in fields.items():
        set_parts.append(f"{k} = %s")
        values.append(v.value if isinstance(v, Priority) else v)
    set_parts.append("updated_at = now()")

    with conn() as c, c.cursor() as cur:
        cur.execute(
            f"""
            UPDATE public.todos
            SET {', '.join(set_parts)}
            WHERE id = %s
            RETURNING id, user_id, category_id, title, title_lower, body, priority, done,
                      deleted_at, created_at, updated_at
            """,
            (*values, todo_id),
        )
        row = cur.fetchone()
        if not row:
            raise HTTPException(status_code=404, detail="todo not found")
        return Todo(**row)


@app.delete("/todos/{todo_id}", status_code=204)
def soft_delete_todo(todo_id: int) -> None:
    """Soft delete — sets deleted_at; the partial index in schema/todos.sql
    keeps the active-todos query fast even with deleted rows around."""
    with conn() as c, c.cursor() as cur:
        cur.execute(
            "UPDATE public.todos SET deleted_at = now() WHERE id = %s AND deleted_at IS NULL",
            (todo_id,),
        )
        if cur.rowcount == 0:
            raise HTTPException(status_code=404, detail="todo not found or already deleted")


# ---------- categories -------------------------------------------------------

@app.get("/categories", response_model=List[Category])
def list_categories() -> List[Category]:
    with conn() as c, c.cursor() as cur:
        cur.execute("SELECT id, name, color FROM public.categories ORDER BY name")
        return [Category(**r) for r in cur.fetchall()]


# ---------- introspection ----------------------------------------------------

@app.get("/_stats/users/{user_id}/todo_count")
def todo_count(user_id: int) -> dict:
    """Calls the SQL function `count_user_todos` declared in schema/functions.sql.
    Demonstrates that pg-flux-managed functions are part of the contract."""
    with conn() as c, c.cursor() as cur:
        cur.execute("SELECT public.count_user_todos(%s) AS total", (user_id,))
        return cur.fetchone()
