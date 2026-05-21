package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// captureAlterTable records RLS flags and/or pass-through partition DDL for ALTER TABLE.
func captureAlterTable(at *pgq.AlterTableStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if at == nil || at.GetRelation() == nil {
		return nil
	}
	sch := at.GetRelation().GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	tname := strings.ToLower(at.GetRelation().GetRelname())
	key := schema.TableKey(sch, tname)

	var rlsOn *bool
	var rlsForce *bool
	needDeparse := false
	// triggerStates collects per-trigger state changes from ALTER TABLE ... ENABLE/DISABLE TRIGGER name.
	// Map key = lowercase trigger name; value = pg_trigger.tgenabled code ("O","D","R","A").
	triggerStates := map[string]string{}
	for _, node := range at.GetCmds() {
		if node == nil {
			continue
		}
		ac := node.GetAlterTableCmd()
		if ac == nil {
			continue
		}
		switch ac.GetSubtype() {
		case pgq.AlterTableType_AT_EnableRowSecurity:
			v := true
			rlsOn = &v
		case pgq.AlterTableType_AT_DisableRowSecurity:
			v := false
			rlsOn = &v
		case pgq.AlterTableType_AT_ForceRowSecurity:
			v := true
			rlsForce = &v
		case pgq.AlterTableType_AT_NoForceRowSecurity:
			v := false
			rlsForce = &v
		case pgq.AlterTableType_AT_AttachPartition, pgq.AlterTableType_AT_DetachPartition, pgq.AlterTableType_AT_DetachPartitionFinalize:
			needDeparse = true
		case pgq.AlterTableType_AT_EnableTrig:
			if n := strings.ToLower(ac.GetName()); n != "" {
				triggerStates[n] = "O"
			}
		case pgq.AlterTableType_AT_EnableAlwaysTrig:
			if n := strings.ToLower(ac.GetName()); n != "" {
				triggerStates[n] = "A"
			}
		case pgq.AlterTableType_AT_EnableReplicaTrig:
			if n := strings.ToLower(ac.GetName()); n != "" {
				triggerStates[n] = "R"
			}
		case pgq.AlterTableType_AT_DisableTrig:
			if n := strings.ToLower(ac.GetName()); n != "" {
				triggerStates[n] = "D"
			}
		}
	}
	for tgName, state := range triggerStates {
		tk := schema.TriggerKey(sch, tname, tgName)
		if existing, ok := st.Triggers[tk]; ok && existing != nil {
			existing.Enabled = state
		}
		// If the trigger isn't captured yet (ALTER TABLE before CREATE TRIGGER in load order),
		// stash the pending state and apply in a second pass.
		if existing := st.Triggers[tk]; existing == nil {
			if st.PendingTriggerState == nil {
				st.PendingTriggerState = map[string]string{}
			}
			st.PendingTriggerState[tk] = state
		}
	}
	if needDeparse {
		sql, err := deparseOne(raw)
		if err != nil {
			return err
		}
		st.ExtraDDL = append(st.ExtraDDL, strings.TrimSpace(sql))
	}
	if t := st.Tables[key]; t != nil {
		if rlsOn != nil {
			t.RLSEnabled = *rlsOn
		}
		if rlsForce != nil {
			t.RLSForced = *rlsForce
		}
	} else if rlsOn != nil || rlsForce != nil {
		// Table not yet loaded (ALTER TABLE in a file sorted before CREATE TABLE).
		// Accumulate pending flags to be applied in a second pass.
		if st.PendingRLS == nil {
			st.PendingRLS = make(map[string]*schema.RLSFlags)
		}
		f := st.PendingRLS[key]
		if f == nil {
			f = &schema.RLSFlags{}
			st.PendingRLS[key] = f
		}
		if rlsOn != nil {
			f.Enabled = *rlsOn
			f.EnabledSet = true
		}
		if rlsForce != nil {
			f.Forced = *rlsForce
			f.ForcedSet = true
		}
	}
	return nil
}
