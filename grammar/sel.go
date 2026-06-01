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
		s("{"),
		grammargen.Repeat(sym("member")),
		s("}"),
	))

	g.Define("member", grammargen.Choice(sym("param"), sym("surface")))

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

	g.Define("statement", grammargen.Choice(sym("let_stmt"), sym("return_stmt")))

	g.Define("let_stmt", seq(
		s("let"),
		field("name", sym("identifier")),
		s("="),
		field("value", sym("expression")),
	))

	g.Define("return_stmt", seq(s("return"), field("value", sym("expression"))))

	// expression is a supertype: the concrete node (binary_expression, call, …)
	// appears directly wherever an expression is expected.
	g.Define("expression", grammargen.Choice(
		sym("binary_expression"),
		sym("member_expression"),
		sym("super_call"),
		sym("call"),
		sym("paren_expression"),
		sym("number"),
		sym("identifier"),
	))

	g.Define("super_call", grammargen.PrecLeft(6, seq(
		s("super"), s("."),
		field("method", sym("identifier")),
		s("("),
		grammargen.Optional(sym("arguments")),
		s(")"),
	)))

	g.Define("binary_expression", grammargen.Choice(
		grammargen.PrecLeft(1, seq(
			field("left", sym("expression")),
			field("operator", grammargen.Choice(s("+"), s("-"))),
			field("right", sym("expression")),
		)),
		grammargen.PrecLeft(2, seq(
			field("left", sym("expression")),
			field("operator", grammargen.Choice(s("*"), s("/"))),
			field("right", sym("expression")),
		)),
	))

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

	g.SetWord("identifier")
	g.SetSupertypes("expression")
	g.SetExtras(grammargen.Pat(`\s`), grammargen.Pat(`//[^\n]*`))

	return g
}
