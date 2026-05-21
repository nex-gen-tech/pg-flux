"""
Database access for the todos service.

Schema is owned by pg-flux: see ../schema/ for the SQL source-of-truth
and ../migrations/ for the generated migration files. This module
opens a connection pool; it does not own the schema.
"""
from __future__ import annotations

import os
from contextlib import contextmanager

import psycopg
from psycopg.rows import dict_row
from psycopg_pool import ConnectionPool


def _dsn() -> str:
    dsn = os.environ.get("DATABASE_URL")
    if not dsn:
        raise RuntimeError("DATABASE_URL is not set")
    return dsn


pool = ConnectionPool(conninfo=_dsn(), min_size=1, max_size=10, kwargs={"row_factory": dict_row})


@contextmanager
def conn():
    with pool.connection() as c:
        yield c
