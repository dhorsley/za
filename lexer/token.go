package lexer

import "fmt"

// Token represents a single lexical token.
type Token struct {
	TokType        int64  // token type from list in constants
	Bindpos        uint64 // binding position
	TokText        string // the content of the token
	TokVal         any    // raw value storage
	LaElseDistance int16  // look ahead markers
	LaEndDistance  int16
	Subtype        uint8  // sub type of identifiers
	Bound          bool
	LaDone         bool
	LaHasElse      bool
}

func (t Token) String() string {
	if t.TokType == StringLiteral {
		return fmt.Sprintf("\"%s\"", t.TokText)
	}
	return t.TokText
}

// Result is the return value of NextToken.
type Result struct {
	Tok    Token
	Pos    int
	Eol    bool
	Eof    bool
	Borpos int
}
