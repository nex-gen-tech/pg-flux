import { z } from 'zod';

// ---- Enums ----

export const BookmarkStatusSchema = z.enum(['unread', 'reading', 'read', 'archived']);
export type BookmarkStatus = z.infer<typeof BookmarkStatusSchema>;

// ---- Users ----

export const UserCreateSchema = z.object({
  email: z.string().email(),
  handle: z.string().min(1).max(50),
});
export type UserCreate = z.infer<typeof UserCreateSchema>;

export interface User {
  id: string;
  email: string;
  handle: string;
  email_verified: boolean;
  created_at: Date;
}

// ---- Collections ----

export const CollectionCreateSchema = z.object({
  name: z.string().min(1).max(100),
  color: z.string().default('blue'),
});
export type CollectionCreate = z.infer<typeof CollectionCreateSchema>;

export interface Collection {
  id: number;
  user_id: string;
  name: string;
  color: string;
}

// ---- Bookmarks ----

export const BookmarkCreateSchema = z.object({
  url: z.string().url().startsWith('http'),
  title: z.string().min(1),
  notes: z.string().max(5000).default(''),
  collection_id: z.number().int().nullable().default(null),
  status: BookmarkStatusSchema.default('unread'),
  metadata: z.record(z.unknown()).default({}),
});
export type BookmarkCreate = z.infer<typeof BookmarkCreateSchema>;

export const BookmarkUpdateSchema = z.object({
  title: z.string().min(1).optional(),
  notes: z.string().max(5000).optional(),
  status: BookmarkStatusSchema.optional(),
  collection_id: z.number().int().nullable().optional(),
  metadata: z.record(z.unknown()).optional(),
});
export type BookmarkUpdate = z.infer<typeof BookmarkUpdateSchema>;

export interface Bookmark {
  id: string;
  user_id: string;
  collection_id: number | null;
  url: string;
  title: string;
  title_lower: string;
  notes: string;
  search_vector: string; // tsvector, returned as string from pg
  metadata: Record<string, unknown>;
  status: BookmarkStatus;
  deleted_at: Date | null;
  created_at: Date;
  updated_at: Date;
}
