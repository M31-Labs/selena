package grammar

import "github.com/odvcencio/gotreesitter/grammargen"

// SelenaGrammar defines the .sel shader-authoring language with grammargen — the
// same engine behind GoSX's .gsx and gosx-native's .swift.gsx. Shape:
//
//	material <Name> {
//	    param <name> : <type> [= <constant expr>]
//	    surface(<geo>) -> <type> {
//	        let <name> = <expr>
//	        return <expr>
//	    }
//	}
//
// Expressions cover binary + - * /, member access (a.b), calls f(args),
// parentheses, identifiers, and number literals, with standard precedence.
func SelenaGrammar() *grammargen.Grammar {
	s := grammargen.Str
	sym := grammargen.Sym
	seq := grammargen.Seq
	field := grammargen.Field

	g := grammargen.NewGrammar("selena")

	g.Define("source_file", grammargen.Repeat(grammargen.Choice(sym("material"), sym("fn_decl"))))

	g.Define("fn_decl", seq(
		s("fn"),
		field("name", sym("identifier")),
		s("("),
		grammargen.Optional(sym("fn_params")),
		s(")"), s("->"),
		field("returns", sym("identifier")),
		field("body", sym("block")),
	))
	g.Define("fn_params", seq(sym("fn_param"), grammargen.Repeat(seq(s(","), sym("fn_param")))))
	g.Define("fn_param", seq(
		field("name", sym("identifier")),
		s(":"),
		field("type", sym("identifier")),
	))

	g.Define("material", seq(
		s("material"),
		field("name", sym("identifier")),
		grammargen.Optional(seq(s("extends"), field("parent", sym("identifier")))),
		grammargen.Optional(seq(s("kind"), field("kind", sym("identifier")))),
		s("{"),
		grammargen.Repeat(sym("member")),
		s("}"),
	))

	// member alternatives: existing (param, surface) first; feedback and
	// statefield APPENDED at the end to preserve LALR symbol-ID order. The new
	// rules themselves are Define-d at the END of this function (after
	// index_assign_stmt) for the same reason.
	// param_array is appended LAST (B3.2: array-typed uniform params).
	// vertex and varying_decl are appended at the very end (B4: author-controlled
	// vertex stage + author-declared vertex->fragment varyings). They MUST stay
	// last in this Choice and be Define-d at the END of the function to preserve
	// the LALR symbol-ID order of every existing rule.
	// context_block is appended LAST of all (the "context uniform" concept: a
	// per-material block of engine-injected uniforms, §3.1 of the context-uniform
	// design). It MUST stay last in this Choice and be Define-d at the very END
	// of the function, after every other rule, to preserve the LALR symbol-ID
	// order of everything above.
	g.Define("member", grammargen.Choice(sym("param"), sym("surface"), sym("feedback"), sym("statefield"), sym("param_array"), sym("vertex"), sym("varying_decl"), sym("context_block")))

	g.Define("param", seq(
		s("param"),
		field("name", sym("identifier")),
		s(":"),
		field("type", sym("identifier")),
		grammargen.Optional(seq(s("="), field("default", sym("expression")))),
	))

	g.Define("surface", seq(
		s("surface"), s("("),
		field("geo", sym("identifier")),
		s(")"), s("->"),
		field("returns", sym("identifier")),
		field("body", sym("block")),
	))

	g.Define("block", seq(s("{"), grammargen.Repeat(sym("statement")), s("}")))

	// statement: existing choices first (let_stmt, return_stmt), then new ones
	// appended at the end to preserve LALR symbol-ID order for existing rules.
	// var_typed_stmt and index_assign_stmt are appended last (B2b arrays).
	// discard_stmt is appended LAST (fragment-only `discard`); its rule is
	// Define-d at the very END of this function for the same LALR ordering reason.
	// _ternary_follow_anchor is appended after discard_stmt: it is a hidden
	// LALR lookahead anchor (NOT a real statement form) that fixes parenthesised
	// and nested ternaries — see its Define at the END for the full rationale.
	g.Define("statement", grammargen.Choice(
		sym("let_stmt"),
		sym("return_stmt"),
		sym("var_stmt"),
		sym("assign_stmt"),
		sym("if_stmt"),
		sym("for_stmt"),
		sym("var_typed_stmt"),
		sym("index_assign_stmt"),
		sym("discard_stmt"),
		sym("break_stmt"),
		sym("_ternary_follow_anchor"),
	))

	g.Define("let_stmt", seq(
		s("let"),
		field("name", sym("identifier")),
		s("="),
		field("value", sym("expression")),
	))

	g.Define("return_stmt", seq(s("return"), field("value", sym("expression"))))

	// expression is a supertype: the concrete node (binary_expression, call, …)
	// appears directly wherever an expression is expected.
	// NOTE: super_call MUST be defined after unary_not_expression and
	// conditional_expression — the definition order affects LALR symbol IDs and
	// therefore the parse-table states. Reordering can silently introduce
	// wrong-parse regressions (e.g. glow-points.sel member_expression spanning
	// two lines).
	// int_literal and uint_literal are appended LAST to preserve existing order.
	// index_expression is appended after uint_literal (B2b arrays).
	g.Define("expression", grammargen.Choice(
		sym("conditional_expression"),
		sym("binary_expression"),
		sym("unary_expression"),
		sym("unary_not_expression"),
		sym("member_expression"),
		sym("super_call"),
		sym("call"),
		sym("paren_expression"),
		sym("number"),
		sym("identifier"),
		sym("int_literal"),
		sym("uint_literal"),
		sym("index_expression"),
	))

	g.Define("binary_expression", grammargen.Choice(
		// Arithmetic: standard * / > + - precedence.
		grammargen.PrecLeft(2, seq(
			field("left", sym("expression")),
			field("operator", grammargen.Choice(s("*"), s("/"))),
			field("right", sym("expression")),
		)),
		grammargen.PrecLeft(1, seq(
			field("left", sym("expression")),
			field("operator", grammargen.Choice(s("+"), s("-"))),
			field("right", sym("expression")),
		)),
		// Comparison operators: looser than arithmetic, result bool.
		// < > <= >= == != are all at the same precedence level (prec 0).
		grammargen.PrecLeft(0, seq(
			field("left", sym("expression")),
			field("operator", grammargen.Choice(s("<="), s(">="), s("=="), s("!="), s("<"), s(">"))),
			field("right", sym("expression")),
		)),
		// Logical AND: looser than comparisons.
		grammargen.PrecLeft(-1, seq(
			field("left", sym("expression")),
			field("operator", s("&&")),
			field("right", sym("expression")),
		)),
		// Logical OR: looser than &&.
		grammargen.PrecLeft(-2, seq(
			field("left", sym("expression")),
			field("operator", s("||")),
			field("right", sym("expression")),
		)),
	))

	// Prefix negation (-). Binds tighter than * / (prec 2) but looser than
	// member access (prec 5), so -a.b is -(a.b) and -a * b is (-a) * b.
	// IMPORTANT: unary_expression and unary_not_expression MUST be two separate
	// rules rather than Choice("-","!") in one rule. Merging them into a single
	// rule via Choice changes LALR state merges in a way that causes wrong
	// parses for expressions like exp(-(x * y * z)) followed by another let on
	// the next line. Keep them split.
	g.Define("unary_expression", grammargen.PrecLeft(4, seq(
		field("operator", s("-")),
		field("operand", sym("expression")),
	)))

	// Prefix logical not (!). Same precedence as negation. Separate rule from
	// unary_expression — see note above.
	g.Define("unary_not_expression", grammargen.PrecLeft(4, seq(
		field("operator", s("!")),
		field("operand", sym("expression")),
	)))

	// Ternary / conditional: cond ? then : alt. Right-associative and looser
	// than all binary operators, so a || b ? c : d is (a || b) ? c : d.
	g.Define("conditional_expression", grammargen.PrecRight(-3, seq(
		field("cond", sym("expression")),
		s("?"),
		field("then", sym("expression")),
		s(":"),
		field("alt", sym("expression")),
	)))

	// super_call MUST be defined after unary_expression, unary_not_expression,
	// and conditional_expression — see note on expression supertype above.
	g.Define("super_call", grammargen.PrecLeft(6, seq(
		s("super"), s("."),
		field("method", sym("identifier")),
		s("("),
		grammargen.Optional(sym("arguments")),
		s(")"),
	)))

	g.Define("member_expression", grammargen.PrecLeft(5, seq(
		field("object", sym("expression")),
		s("."),
		field("field", sym("identifier")),
	)))

	g.Define("call", grammargen.PrecLeft(6, seq(
		field("callee", sym("identifier")),
		s("("),
		grammargen.Optional(sym("arguments")),
		s(")"),
	)))

	g.Define("arguments", seq(
		sym("expression"),
		grammargen.Repeat(seq(s(","), sym("expression"))),
	))

	g.Define("paren_expression", seq(s("("), sym("expression"), s(")")))

	g.Define("identifier", grammargen.Token(grammargen.Pat(`[A-Za-z_][A-Za-z0-9_]*`)))
	g.Define("number", grammargen.Token(grammargen.Pat(`[0-9]+(\.[0-9]+)?`)))

	// Integer literal tokens — defined AFTER number so they do not disturb the
	// existing symbol-ID order. The suffixes 'i' / 'u' make these longer than a
	// bare number match, so the lexer's maximal-munch rule picks them over number
	// when the suffix is present (0i → int_literal, 0u → uint_literal, 0 → number).
	g.Define("int_literal", grammargen.Token(grammargen.Pat(`[0-9]+i`)))
	g.Define("uint_literal", grammargen.Token(grammargen.Pat(`[0-9]+u`)))

	// var_stmt: mutable local declaration  `var name = expr`
	g.Define("var_stmt", seq(
		s("var"),
		field("name", sym("identifier")),
		s("="),
		field("value", sym("expression")),
	))

	// assign_stmt: reassignment of a mutable local  `name = expr`
	g.Define("assign_stmt", seq(
		field("name", sym("identifier")),
		s("="),
		field("value", sym("expression")),
	))

	// if_stmt: `if (cond) { stmts } [else { stmts }]`
	g.Define("if_stmt", seq(
		s("if"),
		s("("),
		field("cond", sym("expression")),
		s(")"),
		field("then", sym("block")),
		grammargen.Optional(seq(s("else"), field("alt", sym("block")))),
	))

	// for_stmt: `for (var init_name = init_val; cond; update_name = update_val) { body }`
	g.Define("for_stmt", seq(
		s("for"),
		s("("),
		s("var"),
		field("init_name", sym("identifier")),
		s("="),
		field("init_val", sym("expression")),
		s(";"),
		field("cond", sym("expression")),
		s(";"),
		field("update_name", sym("identifier")),
		s("="),
		field("update_val", sym("expression")),
		s(")"),
		field("body", sym("block")),
	))

	// array_type: the type annotation for a fixed-size local array.
	// e.g. array<float, 3>  or  array<vec4, 8>
	// NOTE: "array" becomes a reserved keyword; N is a bare integer number literal.
	// ">" is disambiguated from the comparison ">" by parser state: array_type only
	// appears after "var name :" in var_typed_stmt, never inside expression.
	g.Define("array_type", seq(
		s("array"),
		s("<"),
		field("elem", sym("identifier")),
		s(","),
		field("size", sym("number")),
		s(">"),
	))

	// var_typed_stmt: typed local array declaration — `var name : array<T, N>`.
	// No initializer syntax: WGSL zero-initialises; authors fill elements explicitly.
	// Appended after for_stmt to preserve LALR symbol-ID order.
	g.Define("var_typed_stmt", seq(
		s("var"),
		field("name", sym("identifier")),
		s(":"),
		field("typ", sym("array_type")),
	))

	// index_expression: array element read — `expr[index]`.
	// Precedence 6 matches call/super_call (postfix), binds tighter than arithmetic.
	// Appended after uint_literal in expression to preserve LALR symbol-ID order.
	g.Define("index_expression", grammargen.PrecLeft(6, seq(
		field("object", sym("expression")),
		s("["),
		field("index", sym("expression")),
		s("]"),
	)))

	// index_assign_stmt: array element write — `name[index] = value`.
	// Uses identifier (not expression) for the target so it can only write to a
	// named local array, not to a computed lvalue.
	// Appended after var_typed_stmt to preserve LALR symbol-ID order.
	g.Define("index_assign_stmt", seq(
		field("name", sym("identifier")),
		s("["),
		field("index", sym("expression")),
		s("]"),
		s("="),
		field("value", sym("expression")),
	))

	// feedback: the simulation entry for a `kind feedback` material —
	//   feedback(cell) -> vec4 { ... }
	// Structurally mirrors `surface` but its binding is the current cell handle;
	// the body returns the next state vec4 for that cell. The `state(dx, dy)`
	// builtin (a plain call, callee identifier "state") reads the previous state.
	// Defined at the END so its symbol IDs come after all existing rules.
	g.Define("feedback", seq(
		s("feedback"), s("("),
		field("cell", sym("identifier")),
		s(")"), s("->"),
		field("returns", sym("identifier")),
		field("body", sym("block")),
	))

	// statefield: declares a vec4-per-cell state grid — `state <name>`. The host
	// allocates a ping-pong pair (two storage buffers on WebGPU, two float FBOs
	// on WebGL/GLES). `state` is an extracted keyword: in member position it is
	// the statefield keyword; in expression position (e.g. `state(0, 0)`) the
	// word token falls back to identifier so it parses as a call.
	// Defined at the END to preserve LALR symbol-ID order for existing rules.
	g.Define("statefield", seq(
		s("state"),
		field("name", sym("identifier")),
	))

	// param_array_type: type annotation for a fixed-size array uniform param.
	// Structurally identical to array_type but defined separately to avoid LALR
	// state merges between the member context (material) and the statement context
	// (block/for body) that share array_type via var_typed_stmt. B3.2.
	g.Define("param_array_type", seq(
		s("array"),
		s("<"),
		field("elem", sym("identifier")),
		s(","),
		field("size", sym("number")),
		s(">"),
	))

	// param_array: a fixed-size array uniform param — `param name : array<T, N>`.
	// Uses param_array_type (not array_type) to keep contexts disjoint. B3.2.
	// Defined at the END to preserve LALR symbol-ID order for all existing rules.
	// Array params do not support default values (no `= expr` suffix). B3.2.
	g.Define("param_array", seq(
		s("param"),
		field("name", sym("identifier")),
		s(":"),
		field("typ", sym("param_array_type")),
	))

	// vertex: the author-controlled vertex stage for a mesh/general material —
	//   vertex() -> vec4 { ...; return clipPos }
	//   vertex(geo) -> vec4 { ...; return clipPos }
	// Structurally mirrors `surface`, but its geometry binding is OPTIONAL: a bare
	// vertex() builds procedural geometry from the `vertexIndex` builtin (no vertex
	// attributes), while vertex(geo) reads geo.position / geo.normal / geo.uv as
	// per-vertex attributes. The body computes the clip-space position (the return)
	// and writes author-declared varyings via `name = expr` (where name is a
	// `varying`). Defined at the END to preserve LALR symbol-ID order. B4.
	g.Define("vertex", seq(
		s("vertex"), s("("),
		grammargen.Optional(field("geo", sym("identifier"))),
		s(")"), s("->"),
		field("returns", sym("identifier")),
		field("body", sym("block")),
	))

	// varying_decl: an author-declared vertex->fragment varying — `varying name : type`.
	// The vertex() body writes it; the surface() fragment reads it as geo.<name>.
	// Structurally mirrors `param` (minus defaults). Defined at the END to preserve
	// LALR symbol-ID order for all existing rules. B4.
	g.Define("varying_decl", seq(
		s("varying"),
		field("name", sym("identifier")),
		s(":"),
		field("type", sym("identifier")),
	))

	// discard_stmt: the fragment-only `discard` statement. It throws away the
	// current fragment (WGSL/GLSL/GLES `discard;`, Metal `discard_fragment();`).
	// A bare keyword with no fields. Usable inside an if-block like any other
	// statement. Defined at the very END to preserve LALR symbol-ID order for all
	// existing rules (appending it elsewhere broke an existing file before).
	g.Define("discard_stmt", s("discard"))

	// break_stmt: exit the innermost enclosing for loop.
	//
	// Without it, a bounded search has to be written as a fixed-trip loop with a
	// predicated body -- every lane grinds through every iteration even once the answer
	// is found. That is not a stylistic loss: on a GPU the trip count of a ray-march is
	// data-dependent, so a warp whose lanes finish at different steps pays the WORST
	// lane's cost for all of them. The water demo's surface shader marched 30 steps x 64
	// segments per fragment with no way out, and its cost swung with how disturbed the
	// surface was purely because that changed how far the lanes diverged.
	//
	// Appended after discard_stmt for the same LALR symbol-ID ordering reason.
	g.Define("break_stmt", s("break"))

	// _ternary_follow_anchor: a hidden LALR lookahead anchor (NOT a real
	// statement — it never matches valid input). It exists solely to repair a
	// gotreesitter LALR table-generation gap: a conditional_expression
	// (`cond ? then : alt`, a two-infix / three-operand production) does not get
	// the closing delimiters `)` `,` `]` `;` propagated into its reduce lookahead
	// set when it is reached through the `expression` supertype inside a delimited
	// context. The result was that `(a ? b : c)`, `a ? (b ? c : d) : e`,
	// `f(a ? b : c)`, `arr[a ? b : c]`, and ternaries in for-loop clauses failed
	// to parse (the conditional built fine but the enclosing `)` / `]` / `;`
	// could never be shifted). Mentioning `conditional_expression` immediately
	// before each delimiter here forces the LALR generator to add those
	// delimiters to the conditional's FOLLOW/lookahead, which makes the
	// delimited-context reductions exist. Because no valid .sel ever places a
	// bare `conditional_expression` directly before one of these delimiters at
	// statement position (there is no opening bracket on the stack), this anchor
	// is never actually reduced — every existing golden parses byte-identically.
	// Appended to the statement Choice above and Define-d at the very END to
	// preserve the LALR symbol-ID order of all existing rules.
	g.Define("_ternary_follow_anchor", grammargen.Choice(
		seq(sym("conditional_expression"), s(")")),
		seq(sym("conditional_expression"), s(",")),
		seq(sym("conditional_expression"), s("]")),
		seq(sym("conditional_expression"), s(";")),
	))

	// comment: a `//` line comment, declared as a visible named Token rule and
	// wired into extras below. Declaring it as a first-class Token (rather than a
	// bare `Pat("//[^\n]*")` extra) is what makes `//` skip correctly EVERYWHERE.
	// As a plain pattern extra, the lexer in expression-continuation states
	// (e.g. right after `)` in `let a = rgb(1,1,1) // note`) matched the single
	// `/` division operator instead of the longer `//` comment, mis-lexing the
	// comment as a division and turning its words into identifiers (SEL0001 /
	// SEL2001). A Token rule gets first-class maximal-munch priority, so `//...`
	// always wins over `/`.
	//
	// The rule MUST stay visible — it was `_comment`, and a leading underscore
	// hides a rule. Hidden extras produce no node, so their bytes belonged to no
	// child. gotreesitter v0.49.0 added a leaf-tiling invariant that declines any
	// subtree whose span contains bytes no child covers, and from that version on
	// every .sel source containing a comment failed with SEL0001 while
	// comment-free sources still parsed. Visible comment nodes tile the source
	// and satisfy the invariant. See parse/comment_tiling_test.go.
	g.Define("comment", grammargen.Token(grammargen.Pat(`//[^\n]*`)))

	// context_field: one scalar/vector/matrix field inside a `context { ... }`
	// block — `name : type [= default]`. Structurally identical to `param`
	// (:65-71). The optional default is a HOST-OMITTED fallback only
	// (native/test path); unlike a `param` default it never populates
	// customUniforms (see the context-uniform design §3.1).
	//
	// context_field_array is the array-typed counterpart — `name :
	// array<T, N>` — kept as a SEPARATE rule (mirroring param vs param_array,
	// :372) rather than folding both shapes into one "type" field via
	// Choice(identifier, param_array_type): an earlier attempt at the
	// single-rule-with-a-type-Choice form produced a genuine LALR table
	// corruption once a context block mixed several plain fields with an array
	// field. Two disjoint rules, dispatched through context_member below,
	// avoids that specific corruption. Array context fields do not support
	// defaults (matching param_array's B3.2 "no `= expr` suffix" restriction).
	//
	// context_array_type is its OWN array-type rule rather than reusing
	// param_array_type (:359): reusing param_array_type here reproduced the
	// exact LALR state-merge pitfall documented on param_array_type itself
	// ("avoid LALR state merges between the member context ... and the
	// statement context ... that share array_type") — sharing one array-type
	// symbol between the top-level `member` Repeat (via param_array) and this
	// new nested `context_member` Repeat (via context_field_array) corrupted
	// the parse table once a context block had 3+ plain fields before the
	// array field. A disjoint symbol per repeat-context avoids the merge, same
	// as param_array_type itself was split from array_type for this reason.
	//
	// context_member/context_block are appended LAST to the `member` Choice
	// above and every rule here is Defined at the very END of this function,
	// after every existing rule, to preserve the LALR symbol-ID order of
	// everything above (same convention as feedback/statefield/param_array/
	// vertex/varying_decl/discard_stmt).
	g.Define("context_field", seq(
		field("name", sym("identifier")),
		s(":"),
		field("type", sym("identifier")),
		grammargen.Optional(seq(s("="), field("default", sym("expression")))),
	))

	g.Define("context_array_type", seq(
		s("array"),
		s("<"),
		field("elem", sym("identifier")),
		s(","),
		field("size", sym("number")),
		s(">"),
	))

	g.Define("context_field_array", seq(
		field("name", sym("identifier")),
		s(":"),
		field("typ", sym("context_array_type")),
	))

	// context_member: one entry inside a context block — either shape above.
	g.Define("context_member", grammargen.Choice(sym("context_field"), sym("context_field_array")))

	// context_block: the per-material `context { field... }` block declaring
	// engine-injected uniforms — the "context uniform" concept. Its fields lower
	// into the SAME uniform block as params, appended after them, each tagged
	// class:"context" in the compiled bindings.Layout (design §3.2); the
	// emitters need no change because context fields become ordinary uniform
	// members.
	g.Define("context_block", seq(
		s("context"),
		s("{"),
		grammargen.Repeat(sym("context_member")),
		s("}"),
	))

	g.SetWord("identifier")
	g.SetSupertypes("expression")
	g.SetExtras(grammargen.Pat(`\s`), sym("comment"))

	return g
}
