import { Router, Request, Response } from 'express';
import { pool } from '../db';
import { BookmarkCreateSchema, BookmarkUpdateSchema, BookmarkStatusSchema } from '../types';

export const bookmarksRouter = Router({ mergeParams: true });

// POST /users/:userId/bookmarks — create a bookmark
bookmarksRouter.post('/', async (req: Request, res: Response) => {
  const { userId } = req.params as { userId: string };
  const parsed = BookmarkCreateSchema.safeParse(req.body);
  if (!parsed.success) {
    return res.status(400).json({ error: parsed.error.flatten() });
  }

  const { url, title, notes, collection_id, status, metadata } = parsed.data;

  try {
    const result = await pool.query(
      `INSERT INTO public.bookmarks (user_id, url, title, notes, collection_id, status, metadata)
       VALUES ($1, $2, $3, $4, $5, $6, $7)
       RETURNING id, user_id, collection_id, url, title, notes, status, metadata, created_at, updated_at`,
      [userId, url, title, notes, collection_id, status, JSON.stringify(metadata)]
    );
    return res.status(201).json(result.rows[0]);
  } catch (err: unknown) {
    if (isPostgresError(err) && err.code === '23505') {
      return res.status(409).json({ error: 'Unique constraint violation', detail: err.detail });
    }
    if (isPostgresError(err) && err.code === '23514') {
      return res.status(400).json({ error: 'Check constraint violation', detail: err.detail });
    }
    throw err;
  }
});

// GET /users/:userId/bookmarks — list bookmarks (filter by status if provided)
bookmarksRouter.get('/', async (req: Request, res: Response) => {
  const { userId } = req.params as { userId: string };
  const statusParam = req.query['status'] as string | undefined;

  // Validate status if provided
  if (statusParam) {
    const parsed = BookmarkStatusSchema.safeParse(statusParam);
    if (!parsed.success) {
      return res.status(400).json({ error: `Invalid status: ${statusParam}` });
    }
  }

  const query = statusParam
    ? `SELECT id, user_id, collection_id, url, title, notes, status, metadata, created_at, updated_at
       FROM public.bookmarks
       WHERE user_id = $1 AND deleted_at IS NULL AND status = $2
       ORDER BY created_at DESC`
    : `SELECT id, user_id, collection_id, url, title, notes, status, metadata, created_at, updated_at
       FROM public.bookmarks
       WHERE user_id = $1 AND deleted_at IS NULL
       ORDER BY created_at DESC`;

  const params = statusParam ? [userId, statusParam] : [userId];
  const result = await pool.query(query, params);
  return res.json(result.rows);
});

// GET /users/:userId/bookmarks/search — full-text search
bookmarksRouter.get('/search', async (req: Request, res: Response) => {
  const { userId } = req.params as { userId: string };
  const q = req.query['q'] as string | undefined;

  if (!q || q.trim().length === 0) {
    return res.status(400).json({ error: 'Missing required query param: q' });
  }

  const result = await pool.query(
    `SELECT id, user_id, collection_id, url, title, notes, status, metadata, created_at, updated_at
     FROM public.bookmarks
     WHERE user_id = $1
       AND deleted_at IS NULL
       AND search_vector @@ to_tsquery('english', $2)
     ORDER BY ts_rank(search_vector, to_tsquery('english', $2)) DESC`,
    [userId, q]
  );

  return res.json(result.rows);
});

// PATCH /bookmarks/:bookmarkId — partial update
export const bookmarksPatchRouter = Router();
bookmarksPatchRouter.patch('/:bookmarkId', async (req: Request, res: Response) => {
  const { bookmarkId } = req.params;
  const parsed = BookmarkUpdateSchema.safeParse(req.body);
  if (!parsed.success) {
    return res.status(400).json({ error: parsed.error.flatten() });
  }

  const updates = parsed.data;
  const fields = Object.keys(updates) as Array<keyof typeof updates>;
  if (fields.length === 0) {
    return res.status(400).json({ error: 'No fields to update' });
  }

  // Build SET clause dynamically
  const setClauses: string[] = [];
  const values: unknown[] = [];
  let paramIdx = 1;

  for (const field of fields) {
    const value = updates[field];
    if (field === 'metadata') {
      setClauses.push(`${field} = $${paramIdx}`);
      values.push(JSON.stringify(value));
    } else {
      setClauses.push(`${field} = $${paramIdx}`);
      values.push(value);
    }
    paramIdx++;
  }

  values.push(bookmarkId);

  const result = await pool.query(
    `UPDATE public.bookmarks
     SET ${setClauses.join(', ')}
     WHERE id = $${paramIdx} AND deleted_at IS NULL
     RETURNING id, user_id, collection_id, url, title, notes, status, metadata, created_at, updated_at`,
    values
  );

  if (result.rows.length === 0) {
    return res.status(404).json({ error: 'Bookmark not found or already deleted' });
  }

  return res.json(result.rows[0]);
});

// DELETE /bookmarks/:bookmarkId — soft delete
bookmarksPatchRouter.delete('/:bookmarkId', async (req: Request, res: Response) => {
  const { bookmarkId } = req.params;

  const result = await pool.query(
    `UPDATE public.bookmarks
     SET deleted_at = now()
     WHERE id = $1 AND deleted_at IS NULL
     RETURNING id`,
    [bookmarkId]
  );

  if (result.rows.length === 0) {
    return res.status(404).json({ error: 'Bookmark not found or already deleted' });
  }

  return res.status(204).send();
});

// Type guard for PostgreSQL errors
interface PostgresError extends Error {
  code: string;
  detail?: string;
}

function isPostgresError(err: unknown): err is PostgresError {
  return typeof err === 'object' && err !== null && 'code' in err;
}
