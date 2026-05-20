package inspector

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

// loadEventTriggers reads pg_event_trigger and populates SchemaState.EventTriggers.
// Event triggers are database-wide objects, not schema-scoped — they're always
// loaded regardless of the inspected schema list.
func loadEventTriggers(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	if pool == nil || st == nil {
		return nil
	}
	rows, err := pool.Query(ctx, `
		SELECT
			e.evtname,
			e.evtevent,
			-- proc identity as "nspname.proname"
			fn_n.nspname || '.' || fn_p.proname AS proc,
			e.evtenabled::text,
			COALESCE(e.evttags::text[], ARRAY[]::text[]) AS tags,
			COALESCE((SELECT rolname FROM pg_roles r WHERE r.oid = e.evtowner), '') AS owner
		FROM pg_event_trigger e
		JOIN pg_proc        fn_p ON fn_p.oid = e.evtfoid
		JOIN pg_namespace   fn_n ON fn_n.oid = fn_p.pronamespace
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.EventTriggers == nil {
		st.EventTriggers = make(map[string]*schema.EventTrigger)
	}
	for rows.Next() {
		var name, event, proc, enabled, owner string
		var tags []string
		if err := rows.Scan(&name, &event, &proc, &enabled, &tags, &owner); err != nil {
			return err
		}
		et := &schema.EventTrigger{
			Name:     strings.ToLower(name),
			Event:    strings.ToLower(event),
			Function: proc + "()",
			Tags:     tags,
			Owner:    owner,
		}
		// pg_event_trigger.evtenabled: 'O' origin/local (default), 'D' disabled, 'R' replica, 'A' always.
		switch enabled {
		case "D":
			et.Enabled = "DISABLE"
		case "R":
			et.Enabled = "REPLICA"
		case "A":
			et.Enabled = "ALWAYS"
		}
		st.EventTriggers[et.Name] = et
	}
	return rows.Err()
}
