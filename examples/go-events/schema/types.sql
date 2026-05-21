CREATE TYPE public.user_role AS ENUM ('member', 'admin', 'owner');
CREATE TYPE public.event_status AS ENUM ('draft', 'published', 'cancelled');
CREATE TYPE public.attendee_status AS ENUM ('invited', 'confirmed', 'declined', 'waitlisted');
