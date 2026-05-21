import { Router, Request, Response } from 'express';
import { pool } from '../db';
import { UserCreateSchema } from '../types';

export const usersRouter = Router();

// POST /users — create a user
usersRouter.post('/', async (req: Request, res: Response) => {
  const parsed = UserCreateSchema.safeParse(req.body);
  if (!parsed.success) {
    return res.status(400).json({ error: parsed.error.flatten() });
  }

  const { email, handle } = parsed.data;

  try {
    const result = await pool.query(
      `INSERT INTO public.users (email, handle)
       VALUES ($1, $2)
       RETURNING id, email, handle, email_verified, created_at`,
      [email, handle]
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

// GET /users/:userId — get user by UUID id
usersRouter.get('/:userId', async (req: Request, res: Response) => {
  const { userId } = req.params;

  const result = await pool.query(
    `SELECT id, email, handle, email_verified, created_at
     FROM public.users
     WHERE id = $1`,
    [userId]
  );

  if (result.rows.length === 0) {
    return res.status(404).json({ error: 'User not found' });
  }

  return res.json(result.rows[0]);
});

// GET /users/:userId/stats — call count_user_bookmarks SQL function
usersRouter.get('/:userId/stats', async (req: Request, res: Response) => {
  const { userId } = req.params;

  const result = await pool.query(
    `SELECT public.count_user_bookmarks($1) AS total`,
    [userId]
  );

  return res.json({ total: Number(result.rows[0].total) });
});

// Type guard for PostgreSQL errors
interface PostgresError extends Error {
  code: string;
  detail?: string;
}

function isPostgresError(err: unknown): err is PostgresError {
  return typeof err === 'object' && err !== null && 'code' in err;
}
