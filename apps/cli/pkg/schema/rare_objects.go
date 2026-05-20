package schema

// Rare-object info structs. These are populated by the inspector for completeness
// (so a future first-class diff has the data) but are not currently part of the
// structured diff pipeline. Source-side CREATE statements still flow through the
// ExtraDDL/MiscObject pass-through path so they apply on first run.

// OperatorInfo models a user-defined operator (pg_operator entry).
type OperatorInfo struct {
	Schema, Name string
	// LeftType / RightType render via format_type(); empty for prefix/postfix.
	LeftType, RightType string
	Procedure           string // schema.name() of the implementing function
	Owner               string
}

// OperatorClassInfo / OperatorFamilyInfo model pg_opclass / pg_opfamily.
type OperatorClassInfo struct {
	Schema, Name string
	AccessMethod string
	IsDefault    bool
	IntypeSchema string
	Intype       string
	Family       string
	Owner        string
}

type OperatorFamilyInfo struct {
	Schema, Name string
	AccessMethod string
	Owner        string
}

// TS objects (text search).
type TSConfigInfo struct {
	Schema, Name string
	Parser       string // schema.name of the parser
	Owner        string
}
type TSDictInfo struct {
	Schema, Name string
	Template     string
	Options      []string
	Owner        string
}
type TSParserInfo struct {
	Schema, Name                                  string
	Start, Token, End, HeadlineFn, LextypesFn     string
}
type TSTemplateInfo struct {
	Schema, Name string
	InitFn       string
	LexizeFn     string
}

// Cast / Conversion / Transform.
type CastInfo struct {
	SourceType string
	TargetType string
	Function   string // empty for I/O cast or binary-coercible
	Context    string // "implicit" | "assignment" | "explicit"
}
type ConversionInfo struct {
	Schema, Name           string
	ForEncoding, ToEncoding string
	Function               string
	IsDefault              bool
	Owner                  string
}
type TransformInfo struct {
	Type     string // schema.name of the type being transformed
	Language string
	FromSQL  string // function name implementing FROM SQL
	ToSQL    string // function name implementing TO SQL
}

// Language: procedural language registration.
type LanguageInfo struct {
	Name      string
	Trusted   bool
	Handler   string
	Validator string
	Inline    string
	Owner     string
}

// Access method (CREATE ACCESS METHOD).
type AccessMethodInfo struct {
	Name    string
	Type    string // "TABLE" or "INDEX"
	Handler string
}

// Tablespace.
type TablespaceInfo struct {
	Name     string
	Location string
	Owner    string
	Options  []string
}
