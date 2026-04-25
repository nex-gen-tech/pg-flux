-- Table-level UNIQUE and EXCLUDE (contype u / x) for drift coverage.
CREATE TABLE public.t_slots (
    id integer PRIMARY KEY,
    box integer NOT NULL,
    label text,
    CONSTRAINT uq_t_slots_box_lbl UNIQUE (box, label),
    CONSTRAINT ex_t_slots_box EXCLUDE USING btree (box WITH =)
);
