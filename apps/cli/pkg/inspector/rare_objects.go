package inspector

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nex-gen-tech/pg-flux/pkg/obs"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// loadRareObjects populates inspectors for OPERATOR / OPERATOR CLASS / OPERATOR
// FAMILY, TEXT SEARCH CONFIGURATION / DICTIONARY / PARSER / TEMPLATE,
// CAST / CONVERSION / TRANSFORM, LANGUAGE, ACCESS METHOD, TABLESPACE.
//
// These are tracked only for inspection completeness — the differ does NOT
// currently produce structured ALTER for them. Source CREATE statements still
// apply via ExtraDDL/MiscObject pass-through.
//
// Errors on any single query are tolerated (some require superuser), but they
// are now surfaced via obs.Warn so operators can see when a kind was skipped
// rather than the previous silent swallow.
func loadRareObjects(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	if pool == nil || st == nil {
		return nil
	}
	tryLoad(ctx, "operators", func() error { return loadOperators(ctx, pool, st, schemas) })
	tryLoad(ctx, "operator_classes", func() error { return loadOperatorClasses(ctx, pool, st, schemas) })
	tryLoad(ctx, "operator_families", func() error { return loadOperatorFamilies(ctx, pool, st, schemas) })
	tryLoad(ctx, "text_search_configs", func() error { return loadTextSearchConfigs(ctx, pool, st, schemas) })
	tryLoad(ctx, "text_search_dicts", func() error { return loadTextSearchDicts(ctx, pool, st, schemas) })
	tryLoad(ctx, "text_search_parsers", func() error { return loadTextSearchParsers(ctx, pool, st, schemas) })
	tryLoad(ctx, "text_search_templates", func() error { return loadTextSearchTemplates(ctx, pool, st, schemas) })
	tryLoad(ctx, "casts", func() error { return loadCasts(ctx, pool, st) })
	tryLoad(ctx, "conversions", func() error { return loadConversions(ctx, pool, st, schemas) })
	tryLoad(ctx, "transforms", func() error { return loadTransforms(ctx, pool, st) })
	tryLoad(ctx, "languages", func() error { return loadLanguages(ctx, pool, st) })
	tryLoad(ctx, "access_methods", func() error { return loadAccessMethods(ctx, pool, st) })
	tryLoad(ctx, "tablespaces", func() error { return loadTablespaces(ctx, pool, st) })
	return nil
}

// tryLoad invokes fn and surfaces any error via obs.Warn rather than the
// previous silent-swallow with `_ = ...`. Inspector callers downstream still
// receive a (possibly partial) SchemaState — pg-flux deliberately tolerates
// permission errors on the rare-object catalog queries (e.g. pg_subscription
// requires superuser; tablespaces require pg_read_all_settings on PG14-) but
// operators want to see when a kind was skipped.
func tryLoad(ctx context.Context, kind string, fn func() error) {
	if err := fn(); err != nil {
		obs.WarnCtx(ctx, "inspector.rare_object_load_failed",
			"kind", kind,
			"error", err.Error(),
		)
	}
}

func loadOperators(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, o.oprname,
		       COALESCE(format_type(o.oprleft, NULL), '') AS lefty,
		       COALESCE(format_type(o.oprright, NULL), '') AS righty,
		       fn_n.nspname || '.' || p.proname AS proc,
		       COALESCE((SELECT rolname FROM pg_roles WHERE oid = o.oprowner), '') AS owner
		FROM pg_operator o
		JOIN pg_namespace n  ON n.oid = o.oprnamespace
		JOIN pg_proc      p  ON p.oid = o.oprcode
		JOIN pg_namespace fn_n ON fn_n.oid = p.pronamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Operators == nil {
		st.Operators = make(map[string]*schema.OperatorInfo)
	}
	for rows.Next() {
		var nsp, name, lt, rt, proc, owner string
		if err := rows.Scan(&nsp, &name, &lt, &rt, &proc, &owner); err != nil {
			return err
		}
		key := fmt.Sprintf("%s.%s(%s,%s)", strings.ToLower(nsp), name, lt, rt)
		st.Operators[key] = &schema.OperatorInfo{
			Schema: strings.ToLower(nsp), Name: name,
			LeftType: lt, RightType: rt, Procedure: proc, Owner: owner,
		}
	}
	return rows.Err()
}

func loadOperatorClasses(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.opcname, am.amname, c.opcdefault,
		       in_n.nspname, format_type(c.opcintype, NULL) AS intype,
		       fam_n.nspname || '.' || fam.opfname AS family,
		       COALESCE((SELECT rolname FROM pg_roles WHERE oid = c.opcowner), '') AS owner
		FROM pg_opclass c
		JOIN pg_am am          ON am.oid = c.opcmethod
		JOIN pg_namespace n    ON n.oid = c.opcnamespace
		JOIN pg_type t         ON t.oid = c.opcintype
		JOIN pg_namespace in_n ON in_n.oid = t.typnamespace
		JOIN pg_opfamily fam   ON fam.oid = c.opcfamily
		JOIN pg_namespace fam_n ON fam_n.oid = fam.opfnamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.OperatorClasses == nil {
		st.OperatorClasses = make(map[string]*schema.OperatorClassInfo)
	}
	for rows.Next() {
		var nsp, name, am, intypeSch, intype, family, owner string
		var def bool
		if err := rows.Scan(&nsp, &name, &am, &def, &intypeSch, &intype, &family, &owner); err != nil {
			return err
		}
		key := strings.ToLower(nsp) + "." + name + "/" + am
		st.OperatorClasses[key] = &schema.OperatorClassInfo{
			Schema: strings.ToLower(nsp), Name: name, AccessMethod: am, IsDefault: def,
			IntypeSchema: intypeSch, Intype: intype, Family: family, Owner: owner,
		}
	}
	return rows.Err()
}

func loadOperatorFamilies(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, f.opfname, am.amname,
		       COALESCE((SELECT rolname FROM pg_roles WHERE oid = f.opfowner), '') AS owner
		FROM pg_opfamily f
		JOIN pg_am am ON am.oid = f.opfmethod
		JOIN pg_namespace n ON n.oid = f.opfnamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.OperatorFamilies == nil {
		st.OperatorFamilies = make(map[string]*schema.OperatorFamilyInfo)
	}
	for rows.Next() {
		var nsp, name, am, owner string
		if err := rows.Scan(&nsp, &name, &am, &owner); err != nil {
			return err
		}
		key := strings.ToLower(nsp) + "." + name + "/" + am
		st.OperatorFamilies[key] = &schema.OperatorFamilyInfo{
			Schema: strings.ToLower(nsp), Name: name, AccessMethod: am, Owner: owner,
		}
	}
	return rows.Err()
}

func loadTextSearchConfigs(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.cfgname,
		       parser_n.nspname || '.' || p.prsname,
		       COALESCE((SELECT rolname FROM pg_roles WHERE oid = c.cfgowner), '') AS owner
		FROM pg_ts_config c
		JOIN pg_ts_parser p     ON p.oid = c.cfgparser
		JOIN pg_namespace n     ON n.oid = c.cfgnamespace
		JOIN pg_namespace parser_n ON parser_n.oid = p.prsnamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.TSConfigurations == nil {
		st.TSConfigurations = make(map[string]*schema.TSConfigInfo)
	}
	for rows.Next() {
		var nsp, name, parser, owner string
		if err := rows.Scan(&nsp, &name, &parser, &owner); err != nil {
			return err
		}
		st.TSConfigurations[strings.ToLower(nsp)+"."+name] = &schema.TSConfigInfo{
			Schema: strings.ToLower(nsp), Name: name, Parser: parser, Owner: owner,
		}
	}
	return rows.Err()
}

func loadTextSearchDicts(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, d.dictname,
		       tn.nspname || '.' || t.tmplname AS template,
		       COALESCE(d.dictinitoption, '') AS opts,
		       COALESCE((SELECT rolname FROM pg_roles WHERE oid = d.dictowner), '') AS owner
		FROM pg_ts_dict d
		JOIN pg_ts_template t ON t.oid = d.dicttemplate
		JOIN pg_namespace n   ON n.oid = d.dictnamespace
		JOIN pg_namespace tn  ON tn.oid = t.tmplnamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.TSDictionaries == nil {
		st.TSDictionaries = make(map[string]*schema.TSDictInfo)
	}
	for rows.Next() {
		var nsp, name, tpl, opts, owner string
		if err := rows.Scan(&nsp, &name, &tpl, &opts, &owner); err != nil {
			return err
		}
		var optList []string
		if opts != "" {
			optList = []string{opts}
		}
		st.TSDictionaries[strings.ToLower(nsp)+"."+name] = &schema.TSDictInfo{
			Schema: strings.ToLower(nsp), Name: name, Template: tpl, Options: optList, Owner: owner,
		}
	}
	return rows.Err()
}

func loadTextSearchParsers(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, p.prsname
		FROM pg_ts_parser p
		JOIN pg_namespace n ON n.oid = p.prsnamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.TSParsers == nil {
		st.TSParsers = make(map[string]*schema.TSParserInfo)
	}
	for rows.Next() {
		var nsp, name string
		if err := rows.Scan(&nsp, &name); err != nil {
			return err
		}
		st.TSParsers[strings.ToLower(nsp)+"."+name] = &schema.TSParserInfo{
			Schema: strings.ToLower(nsp), Name: name,
		}
	}
	return rows.Err()
}

func loadTextSearchTemplates(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, t.tmplname
		FROM pg_ts_template t
		JOIN pg_namespace n ON n.oid = t.tmplnamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.TSTemplates == nil {
		st.TSTemplates = make(map[string]*schema.TSTemplateInfo)
	}
	for rows.Next() {
		var nsp, name string
		if err := rows.Scan(&nsp, &name); err != nil {
			return err
		}
		st.TSTemplates[strings.ToLower(nsp)+"."+name] = &schema.TSTemplateInfo{
			Schema: strings.ToLower(nsp), Name: name,
		}
	}
	return rows.Err()
}

func loadCasts(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	rows, err := pool.Query(ctx, `
		SELECT format_type(c.castsource, NULL), format_type(c.casttarget, NULL),
		       COALESCE(fn_n.nspname || '.' || p.proname, '') AS proc,
		       c.castcontext::text
		FROM pg_cast c
		LEFT JOIN pg_proc      p    ON p.oid = c.castfunc
		LEFT JOIN pg_namespace fn_n ON fn_n.oid = p.pronamespace
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Casts == nil {
		st.Casts = make(map[string]*schema.CastInfo)
	}
	for rows.Next() {
		var src, dst, proc, ctx string
		if err := rows.Scan(&src, &dst, &proc, &ctx); err != nil {
			return err
		}
		ci := &schema.CastInfo{SourceType: src, TargetType: dst, Function: proc}
		switch ctx {
		case "i":
			ci.Context = "implicit"
		case "a":
			ci.Context = "assignment"
		case "e":
			ci.Context = "explicit"
		}
		st.Casts[src+"=>"+dst] = ci
	}
	return rows.Err()
}

func loadConversions(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.conname,
		       pg_encoding_to_char(c.conforencoding),
		       pg_encoding_to_char(c.contoencoding),
		       fn_n.nspname || '.' || p.proname AS proc,
		       c.condefault,
		       COALESCE((SELECT rolname FROM pg_roles WHERE oid = c.conowner), '') AS owner
		FROM pg_conversion c
		JOIN pg_namespace n  ON n.oid = c.connamespace
		JOIN pg_proc      p  ON p.oid = c.conproc
		JOIN pg_namespace fn_n ON fn_n.oid = p.pronamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Conversions == nil {
		st.Conversions = make(map[string]*schema.ConversionInfo)
	}
	for rows.Next() {
		var nsp, name, forEnc, toEnc, proc, owner string
		var def bool
		if err := rows.Scan(&nsp, &name, &forEnc, &toEnc, &proc, &def, &owner); err != nil {
			return err
		}
		st.Conversions[strings.ToLower(nsp)+"."+name] = &schema.ConversionInfo{
			Schema: strings.ToLower(nsp), Name: name,
			ForEncoding: forEnc, ToEncoding: toEnc, Function: proc, IsDefault: def, Owner: owner,
		}
	}
	return rows.Err()
}

func loadTransforms(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	rows, err := pool.Query(ctx, `
		SELECT format_type(t.trftype, NULL), l.lanname,
		       COALESCE(fn1_n.nspname || '.' || p1.proname, '') AS fromsql,
		       COALESCE(fn2_n.nspname || '.' || p2.proname, '') AS tosql
		FROM pg_transform t
		JOIN pg_language l ON l.oid = t.trflang
		LEFT JOIN pg_proc p1    ON p1.oid = t.trffromsql
		LEFT JOIN pg_namespace fn1_n ON fn1_n.oid = p1.pronamespace
		LEFT JOIN pg_proc p2    ON p2.oid = t.trftosql
		LEFT JOIN pg_namespace fn2_n ON fn2_n.oid = p2.pronamespace
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Transforms == nil {
		st.Transforms = make(map[string]*schema.TransformInfo)
	}
	for rows.Next() {
		var typ, lang, from, to string
		if err := rows.Scan(&typ, &lang, &from, &to); err != nil {
			return err
		}
		st.Transforms[typ+"/"+lang] = &schema.TransformInfo{
			Type: typ, Language: lang, FromSQL: from, ToSQL: to,
		}
	}
	return rows.Err()
}

func loadLanguages(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	rows, err := pool.Query(ctx, `
		SELECT l.lanname, l.lanpltrusted,
		       COALESCE((SELECT proname FROM pg_proc WHERE oid = l.lanplcallfoid), '') AS handler,
		       COALESCE((SELECT proname FROM pg_proc WHERE oid = l.lanvalidator), '') AS validator,
		       COALESCE((SELECT proname FROM pg_proc WHERE oid = l.laninline), '') AS inline,
		       COALESCE((SELECT rolname FROM pg_roles WHERE oid = l.lanowner), '') AS owner
		FROM pg_language l
		WHERE l.lanispl
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Languages == nil {
		st.Languages = make(map[string]*schema.LanguageInfo)
	}
	for rows.Next() {
		var name, handler, validator, inline, owner string
		var trusted bool
		if err := rows.Scan(&name, &trusted, &handler, &validator, &inline, &owner); err != nil {
			return err
		}
		st.Languages[strings.ToLower(name)] = &schema.LanguageInfo{
			Name: strings.ToLower(name), Trusted: trusted,
			Handler: handler, Validator: validator, Inline: inline, Owner: owner,
		}
	}
	return rows.Err()
}

func loadAccessMethods(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	rows, err := pool.Query(ctx, `
		SELECT a.amname,
		       CASE a.amtype WHEN 't' THEN 'TABLE' WHEN 'i' THEN 'INDEX' ELSE a.amtype::text END,
		       COALESCE((SELECT proname FROM pg_proc WHERE oid = a.amhandler), '') AS handler
		FROM pg_am a
		WHERE a.amname NOT IN ('btree','hash','gist','gin','spgist','brin','heap')
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.AccessMethods == nil {
		st.AccessMethods = make(map[string]*schema.AccessMethodInfo)
	}
	for rows.Next() {
		var name, typ, handler string
		if err := rows.Scan(&name, &typ, &handler); err != nil {
			return err
		}
		st.AccessMethods[strings.ToLower(name)] = &schema.AccessMethodInfo{
			Name: strings.ToLower(name), Type: typ, Handler: handler,
		}
	}
	return rows.Err()
}

func loadTablespaces(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	rows, err := pool.Query(ctx, `
		SELECT t.spcname,
		       COALESCE(pg_tablespace_location(t.oid), '') AS loc,
		       COALESCE((SELECT rolname FROM pg_roles WHERE oid = t.spcowner), '') AS owner,
		       COALESCE(t.spcoptions::text[], ARRAY[]::text[]) AS opts
		FROM pg_tablespace t
		WHERE t.spcname NOT IN ('pg_default','pg_global')
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Tablespaces == nil {
		st.Tablespaces = make(map[string]*schema.TablespaceInfo)
	}
	for rows.Next() {
		var name, loc, owner string
		var opts []string
		if err := rows.Scan(&name, &loc, &owner, &opts); err != nil {
			return err
		}
		st.Tablespaces[strings.ToLower(name)] = &schema.TablespaceInfo{
			Name: strings.ToLower(name), Location: loc, Owner: owner, Options: opts,
		}
	}
	return rows.Err()
}
