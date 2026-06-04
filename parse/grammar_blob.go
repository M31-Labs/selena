package parse

import _ "embed"

//go:generate go run ../cmd/selena-grammar

// grammarBlob is the pre-generated parse table for grammar.SelenaGrammar().
// TestGrammarBinIsCurrent guards it against drift.
//
//go:embed grammar.bin
var grammarBlob []byte
