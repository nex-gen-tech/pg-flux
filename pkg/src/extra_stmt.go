package src

import (
	"fmt"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nexg/pg-flux/pkg/schema"
)

func deparseOne(raw *pgq.RawStmt) (string, error) {
	if raw == nil {
		return "", fmt.Errorf("nil statement")
	}
	return pgq.Deparse(&pgq.ParseResult{Stmts: []*pgq.RawStmt{raw}})
}

// processExtraNode handles Index, Function, Policy DDL.
func processExtraNode(raw *pgq.RawStmt, st *schema.SchemaState, opt LoadOptions) error {
	if raw == nil || raw.GetStmt() == nil {
		return nil
	}
	switch n := raw.GetStmt().GetNode().(type) {
	case *pgq.Node_IndexStmt:
		return captureIndex(n.IndexStmt, raw, st)
	case *pgq.Node_CreateFunctionStmt:
		if err := captureFunction(n.CreateFunctionStmt, raw, st); err != nil {
			return err
		}
		if opt.ValidatePlpgsql {
			sql, e := deparseOne(raw)
			if e != nil {
				return e
			}
			if strings.Contains(strings.ToLower(sql), "language plpgsql") {
				if e := CheckPlPgSqlSource(sql); e != nil {
					return fmt.Errorf("plpgsql validation: %w", e)
				}
			}
		}
		return nil
	case *pgq.Node_CreatePolicyStmt:
		return capturePolicy(n.CreatePolicyStmt, raw, st)
	case *pgq.Node_AlterPolicyStmt:
		return captureAlterPolicy(n.AlterPolicyStmt, raw, st)
	case *pgq.Node_CreateExtensionStmt:
		return captureExtension(n.CreateExtensionStmt, raw, st)
	case *pgq.Node_AlterExtensionStmt:
		return captureAlterExtension(n.AlterExtensionStmt, raw, st)
	case *pgq.Node_CreateForeignServerStmt:
		return captureDeparsedMisc("FDW_SERVER", raw, st)
	case *pgq.Node_CreateForeignTableStmt:
		return captureDeparsedMisc("FOREIGN_TABLE", raw, st)
	case *pgq.Node_CreatePublicationStmt:
		return captureDeparsedMisc("PUBLICATION", raw, st)
	// PRD v2 / V2-A: type and schema DDL must not be silently dropped (pass-through in plan order).
	case *pgq.Node_CreateDomainStmt:
		// Capture structured domain data for constraint diffing, AND keep the raw DDL.
		if err := captureDomain(n.CreateDomainStmt, st); err != nil {
			return err
		}
		return captureDeparsedExtraDDL(raw, st)
	case *pgq.Node_DefineStmt, *pgq.Node_CompositeTypeStmt, *pgq.Node_CreateEnumStmt, *pgq.Node_CreateSchemaStmt, *pgq.Node_AlterTypeStmt:
		return captureDeparsedExtraDDL(raw, st)
	// GRANT / REVOKE: pass-through as MiscObject so they are applied in plan order.
	case *pgq.Node_GrantStmt, *pgq.Node_GrantRoleStmt:
		return captureDeparsedMisc("GRANT", raw, st)
	// COMMENT ON ... IS '...' — set the Comment field on the target object.
	case *pgq.Node_CommentStmt:
		return captureComment(n.CommentStmt, st)
	default:
		return nil
	}
}

func captureDeparsedExtraDDL(raw *pgq.RawStmt, st *schema.SchemaState) error {
	if raw == nil {
		return nil
	}
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	s := strings.TrimSpace(sql)
	if s == "" {
		return nil
	}
	st.ExtraDDL = append(st.ExtraDDL, s)
	return nil
}

// captureDomain extracts structured domain information into SchemaState.Domains.
func captureDomain(d *pgq.CreateDomainStmt, st *schema.SchemaState) error {
	if d == nil || d.GetDomainname() == nil {
		return nil
	}
	names := d.GetDomainname()
	var sch, name string
	switch len(names) {
	case 1:
		sch = "public"
		name = strings.ToLower(names[0].GetString_().GetSval())
	case 2:
		sch = strings.ToLower(names[0].GetString_().GetSval())
		name = strings.ToLower(names[1].GetString_().GetSval())
	default:
		return nil
	}
	key := sch + "." + name

	// Extract base type name.
	baseType := ""
	if tn := d.GetTypeName(); tn != nil {
		if parts := tn.GetNames(); len(parts) > 0 {
			var typParts []string
			for _, p := range parts {
				s := p.GetString_().GetSval()
				if s != "pg_catalog" {
					typParts = append(typParts, s)
				}
			}
			baseType = strings.Join(typParts, ".")
		}
	}

	dom := &schema.Domain{Schema: sch, Name: name, BaseType: baseType}

	// Extract CHECK constraints.
	for _, cn := range d.GetConstraints() {
		cc := cn.GetConstraint()
		if cc == nil || cc.GetContype() != pgq.ConstrType_CONSTR_CHECK {
			continue
		}
		expr := ""
		if raw := cc.GetRawExpr(); raw != nil {
			if s, err := deparseExprToSQL(raw); err == nil {
				expr = strings.TrimSpace(s)
			}
		}
		if expr != "" {
			dom.Constraints = append(dom.Constraints, schema.DomainConstraint{
				Name: cc.GetConname(),
				Expr: expr,
			})
		}
	}

	if st.Domains == nil {
		st.Domains = make(map[string]*schema.Domain)
	}
	st.Domains[key] = dom
	return nil
}

func captureIndex(x *pgq.IndexStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if x == nil {
		return nil
	}
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	iname := strings.ToLower(x.GetIdxname())
	r := x.GetRelation()
	if r == nil {
		return nil
	}
	sch := r.GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	tbl := strings.ToLower(r.GetRelname())
	key := schema.IndexKey(sch, iname)
	ensureObjectMaps(st)
	if st.Indexes[key] != nil {
		return fmt.Errorf("duplicate index %q", key)
	}
	st.Indexes[key] = &schema.Index{
		Schema:      sch,
		Name:        iname,
		TableSchema: sch,
		Table:       tbl,
		CreateSQL:   sql,
		Concurrent:  x.GetConcurrent(),
	}
	return nil
}

func ensureObjectMaps(st *schema.SchemaState) {
	if st.Indexes == nil {
		st.Indexes = make(map[string]*schema.Index)
	}
	if st.Functions == nil {
		st.Functions = make(map[string]*schema.Function)
	}
	if st.Policies == nil {
		st.Policies = make(map[string]*schema.Policy)
	}
	if st.Extensions == nil {
		st.Extensions = make(map[string]*schema.Extension)
	}
	ensureMoreMaps(st)
}

func captureFunction(x *pgq.CreateFunctionStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if x == nil {
		return nil
	}
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	schemaName, name := nameFromNameList(x.GetFuncname())
	if name == "" {
		return nil
	}
	identity := functionIdentityString(schemaName, x)
	fp, _ := pgq.Fingerprint(strings.TrimSpace(sql))
	ensureObjectMaps(st)
	fk := schema.FunctionKey(identity)
	if st.Functions[fk] != nil {
		return fmt.Errorf("duplicate function %q", fk)
	}
	st.Functions[fk] = &schema.Function{
		Schema:      schemaName,
		Name:        name,
		Language:    "sql",
		Kind:        "f",
		DefSQL:      sql,
		Identity:    identity,
		Fingerprint: fp,
	}
	return nil
}

func functionIdentityString(schemaName string, x *pgq.CreateFunctionStmt) string {
	if schemaName == "" {
		schemaName = "public"
	}
	var name string
	if n := x.GetFuncname(); len(n) > 0 {
		if s := n[len(n)-1].GetString_(); s != nil {
			name = strings.ToLower(s.GetSval())
		}
	}
	var atypes []string
	for _, p := range x.GetParameters() {
		if p == nil {
			continue
		}
		if fp := p.GetFunctionParameter(); fp != nil {
			// Skip OUT / TABLE parameters — they are not part of the function's
			// proargtypes identity (which only counts input arguments), matching
			// what the inspector reads from pg_proc.proargtypes.
			mode := fp.GetMode()
			if mode == pgq.FunctionParameterMode_FUNC_PARAM_OUT ||
				mode == pgq.FunctionParameterMode_FUNC_PARAM_TABLE {
				continue
			}
			if ts, e := typeNameToSQL(fp.GetArgType()); e == nil {
				atypes = append(atypes, schema.NormalizeTypeForCompare(ts))
			}
		}
	}
	return schema.BuildFunctionIdentity(schemaName, name, strings.Join(atypes, ", "))
}

func nameFromNameList(nodes []*pgq.Node) (string, string) {
	if len(nodes) == 0 {
		return "public", ""
	}
	if len(nodes) == 1 {
		if s := nodes[0].GetString_(); s != nil {
			return "public", strings.ToLower(s.GetSval())
		}
		return "public", ""
	}
	schemaName := "public"
	if s := nodes[0].GetString_(); s != nil {
		schemaName = strings.ToLower(s.GetSval())
	}
	if s := nodes[len(nodes)-1].GetString_(); s != nil {
		return schemaName, strings.ToLower(s.GetSval())
	}
	return "public", ""
}

func captureExtension(x *pgq.CreateExtensionStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
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
	if st.Extensions[k] != nil {
		return fmt.Errorf("duplicate extension %q", k)
	}
	ver := ExtensionVersionFromDefSQL(sql)
	st.Extensions[k] = &schema.Extension{Name: name, DefSQL: sql, Version: ver}
	return nil
}

func captureDeparsedMisc(kind string, raw *pgq.RawStmt, st *schema.SchemaState) error {
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	st.MiscObjects = append(st.MiscObjects, &schema.MiscObject{Kind: kind, DefSQL: sql, Name: kind})
	return nil
}

func capturePolicy(x *pgq.CreatePolicyStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if x == nil || x.GetTable() == nil {
		return nil
	}
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	tb := x.GetTable()
	sch := tb.GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	rel := strings.ToLower(tb.GetRelname())
	pn := x.GetPolicyName()
	ensureObjectMaps(st)
	k := schema.PolicyKey(sch, rel, pn)
	if st.Policies[k] != nil {
		return fmt.Errorf("duplicate policy %q", k)
	}
	pol := &schema.Policy{
		Schema:     sch,
		Table:      rel,
		Name:       pn,
		Cmd:        x.GetCmdName(),
		DefSQL:     sql,
		Permissive: x.GetPermissive(),
		UsingSQL:   deparseExprNode(x.GetQual()),
		WithCheck:  deparseExprNode(x.GetWithCheck()),
	}
	pol.Roles = policyRolesFromNodes(x.GetRoles())
	st.Policies[k] = pol
	_ = raw
	return nil
}

// deparseExprNode deparsess a single expression node by wrapping it in a synthetic
// SELECT statement, then stripping the "SELECT " prefix from the result.
func deparseExprNode(n *pgq.Node) string {
	if n == nil {
		return ""
	}
	synth := &pgq.ParseResult{
		Stmts: []*pgq.RawStmt{
			{
				Stmt: &pgq.Node{
					Node: &pgq.Node_SelectStmt{
						SelectStmt: &pgq.SelectStmt{
							TargetList: []*pgq.Node{
								pgq.MakeResTargetNodeWithVal(n, 0),
							},
						},
					},
				},
			},
		},
	}
	dep, err := pgq.Deparse(synth)
	if err != nil {
		return ""
	}
	dep = strings.ToLower(strings.TrimSpace(dep))
	return strings.TrimPrefix(dep, "select ")
}

func captureAlterPolicy(x *pgq.AlterPolicyStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if x == nil || x.GetTable() == nil {
		return nil
	}
	tb := x.GetTable()
	sch := tb.GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	rel := strings.ToLower(tb.GetRelname())
	pn := x.GetPolicyName()
	ensureObjectMaps(st)
	k := schema.PolicyKey(sch, rel, pn)
	prev := st.Policies[k]
	if prev == nil {
		// CREATE POLICY not yet parsed (cross-file ordering) — buffer for second pass.
		p := &schema.PendingAlterPol{Key: k}
		if x.GetQual() != nil {
			if u, e := deparseExprToSQL(x.GetQual()); e == nil {
				p.UsingSQL = u
			}
		}
		if x.GetWithCheck() != nil {
			if w, e := deparseExprToSQL(x.GetWithCheck()); e == nil {
				p.WithCheck = w
			}
		}
		if r := policyRolesFromNodes(x.GetRoles()); len(r) > 0 {
			p.Roles = r
		}
		st.PendingAlterPolicy = append(st.PendingAlterPolicy, p)
		return nil
	}
	applyAlterPolicyToPrev(prev, x)
	return nil
}

// applyAlterPolicyToPrev merges the ALTER POLICY changes into the existing desired policy.
// DefSQL is rebuilt as a valid CREATE POLICY statement so that ChangeCreatePolicy
// generates correct DDL (not an ALTER POLICY statement).
func applyAlterPolicyToPrev(prev *schema.Policy, x *pgq.AlterPolicyStmt) {
	if x.GetQual() != nil {
		if u, e := deparseExprToSQL(x.GetQual()); e == nil {
			prev.UsingSQL = u
		}
	}
	if x.GetWithCheck() != nil {
		if w, e := deparseExprToSQL(x.GetWithCheck()); e == nil {
			prev.WithCheck = w
		}
	}
	if r := policyRolesFromNodes(x.GetRoles()); len(r) > 0 {
		prev.Roles = r
	}
	prev.DefSQL = rebuildCreatePolicySQL(prev)
}

// rebuildCreatePolicySQL constructs a CREATE POLICY statement from a Policy's individual
// fields. Used when ALTER POLICY modifies an existing desired policy so that DefSQL
// stays a valid CREATE POLICY (ChangeCreatePolicy uses DefSQL as the migration DDL).
func rebuildCreatePolicySQL(p *schema.Policy) string {
	var sb strings.Builder
	sb.WriteString("CREATE POLICY ")
	sb.WriteString(p.Name)
	sb.WriteString(" ON ")
	if p.Schema != "" {
		sb.WriteString(p.Schema)
		sb.WriteString(".")
	}
	sb.WriteString(p.Table)
	if !p.Permissive {
		sb.WriteString(" AS RESTRICTIVE")
	}
	cmd := strings.ToLower(p.Cmd)
	if cmd != "" && cmd != "*" && cmd != "all" {
		sb.WriteString(" FOR ")
		sb.WriteString(strings.ToUpper(cmd))
	}
	if len(p.Roles) > 0 {
		sb.WriteString(" TO ")
		sb.WriteString(strings.Join(p.Roles, ", "))
	}
	if p.UsingSQL != "" {
		sb.WriteString(" USING (")
		sb.WriteString(p.UsingSQL)
		sb.WriteString(")")
	}
	if p.WithCheck != "" {
		sb.WriteString(" WITH CHECK (")
		sb.WriteString(p.WithCheck)
		sb.WriteString(")")
	}
	return sb.String()
}
