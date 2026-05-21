import { Router, Request, Response } from 'express';
import { pool } from '../db';
import { CollectionCreateSchema } from '../types';

export const collectionsRouter = Router({ mergeParams: true });

// POST /users/:userId/collections — create a collection
collectionsRouter.post('/', async (req: Request, res: Response) => {
  const { userId } = req.params as { userId: string };
  const parsed = CollectionCreateSchema.safeParse(req.body);
  if (!parsed.success) {
    return res.status(400).json({ error: parsed.error.flatten() });
  }

  const { name, color } = parsed.data;

  try {
    const result = await pool.query(
      `INSERT INTO public.collections (user_id, name, color)
       VALUES ($1, $2, $3)
       RETURNING id, user_id, name, color`,
      [userId, name, color]
    );
    return res.status(201).json(result.rows[0]);
  } catch (err: unknown) {
    if (isPostgresError(err) && err.code === '23505') {
      return res.status(409).json({
        error: 'Unique constraint violation',
        detail: err.detail,
      });
    }
    throw err;
  }
});

// GET /users/:userId/collections — list collections for user
collectionsRouter.get('/', async (req: Request, res: Response) => {
  const { userId } = req.params as { userId: string };

  const result = await pool.query(
    `SELECT id, user_id, name, color
     FROM public.collections
     WHERE user_id = $1
     ORDER BY name ASC`,
    [userId]
  );

  return res.json(result.rows);
});

// Type guard for PostgreSQL errors
interface PostgresError extends Error {
  code: string;
  detail?: string;
}

function isPostgresError(err: unknown): err is PostgresError {
  return typeof err === 'object' && err !== null && 'code' in err;
}
