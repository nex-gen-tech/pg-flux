package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nexg/pg-flux/pkg/schema"
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
	}
	return nil
}
