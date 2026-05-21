import express from 'express';
import dotenv from 'dotenv';

import { usersRouter } from './routes/users';
import { bookmarksRouter } from './routes/bookmarks';
import { bookmarksPatchRouter } from './routes/bookmarks';
import { collectionsRouter } from './routes/collections';

dotenv.config();

const app = express();
app.use(express.json());

// User routes
app.use('/users', usersRouter);

// Nested routes under /users/:userId
app.use('/users/:userId/bookmarks', bookmarksRouter);
app.use('/users/:userId/collections', collectionsRouter);

// Standalone bookmark routes (patch/delete by ID)
app.use('/bookmarks', bookmarksPatchRouter);

// Health check
app.get('/health', (_req, res) => {
  res.json({ status: 'ok' });
});

// Global error handler
app.use((err: Error, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
  console.error(err);
  res.status(500).json({ error: 'Internal server error', message: err.message });
});

const PORT = process.env['PORT'] ?? 8012;
app.listen(PORT, () => {
  console.log(`express-bookmarks listening on http://localhost:${PORT}`);
});

export default app;
