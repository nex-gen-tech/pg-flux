package src

import (
	"regexp"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

var reAlterExtUpdateTo = regexp.MustCompile(`(?i)\bUPDATE\s+TO\s+('[^']*'|"[^"]*"|[^\s;]+)`)

func extensionUpdateToVersion(sql string) string {
	sub := reAlterExtUpdateTo.FindStringSubmatch(sql)
	if len(sub) < 2 {
		return ""
	}
	v := strings.TrimSpace(sub[1])
	return strings.Trim(v, `'"`)
}

// captureAlterExtension merges ALTER EXTENSION ... UPDATE into the extension entry (opt-in desired state).
func captureAlterExtension(x *pgq.AlterExtensionStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if x == nil {
		return nil
	}
	name := strings.ToLower(strings.TrimSpace(x.GetExtname()))
	if name == "" {
		return nil
	}
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	ensureObjectMaps(st)
	if st.Extensions == nil {
		st.Extensions = make(map[string]*schema.Extension)
	}
	k := schema.ExtensionKey(name)
	ver := extensionUpdateToVersion(sql)
	prev := st.Extensions[k]
	if prev == nil {
		st.Extensions[k] = &schema.Extension{Name: name, DefSQL: sql, Version: ver}
		return nil
	}
	prev.DefSQL = sql
	if ver != "" {
		prev.Version = ver
	}
	return nil
}
