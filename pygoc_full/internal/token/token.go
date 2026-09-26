// Package token defines every kind of token PyGo's lexer can produce.
//
// Design note: token types live in their own tiny package (rather than
// inside `lexer`) because both the lexer AND the parser need to refer to
// token kinds, and this avoids a circular import between them.
package token

// Type identifies what kind of token this is.
type Type int

const (
	// Special / structural tokens — these do NOT correspond to a single
	// character in the source. They are synthesized by the lexer.
	ILLEGAL Type = iota // a character/sequence we don't recognize
	EOF                 // end of file
	NEWLINE             // end of a logical line (statement terminator)
	INDENT              // a new, deeper indentation block has started
	DEDENT              // an indentation block has ended

	// Literals
	IDENT  // x, foo, myVar
	INT    // 123
	FLOAT  // 3.14
	STRING // "hello"

	// Keywords
	AND
	OR
	NOT
	TRUE
	FALSE
	IF
	ELIF
	ELSE
	WHILE
	FOR
	IN
	RANGE
	DEF
	RETURN
	BREAK
	CONTINUE

	// Operators
	PLUS    // +
	MINUS   // -
	STAR    // *
	SLASH   // /
	PERCENT // %
	DSLASH  // //
	ASSIGN  // =
	PLUSEQ  // +=
	MINUSEQ // -=
	STAREQ  // *=
	SLASHEQ // /=
	EQ      // ==
	NEQ     // !=
	LT      // <
	GT      // >
	LE      // <=
	GE      // >=
	ARROW   // ->

	// Delimiters
	LPAREN   // (
	RPAREN   // )
	LBRACKET // [
	RBRACKET // ]
	LBRACE   // {
	RBRACE   // }
	COMMA    // ,
	COLON    // :
	DOT      // .
)

// Token is one lexical unit produced by the lexer.
//
// Line and Col are 1-indexed and point at the FIRST character of the
// token, so diagnostics ("error at line 4, col 9") can be built directly
// from a Token without any extra bookkeeping.
type Token struct {
	Type    Type
	Lexeme  string // the raw source text, e.g. "123", "x", "+="
	Literal any    // decoded value for INT/FLOAT/STRING; nil otherwise
	Line    int
	Col     int
}

// keywords maps reserved words to their token type. Anything not in this
// map that looks like an identifier is just an IDENT.
var keywords = map[string]Type{
	"and":      AND,
	"or":       OR,
	"not":      NOT,
	"True":     TRUE,
	"False":    FALSE,
	"if":       IF,
	"elif":     ELIF,
	"else":     ELSE,
	"while":    WHILE,
	"for":      FOR,
	"in":       IN,
	"range":    RANGE,
	"def":      DEF,
	"return":   RETURN,
	"break":    BREAK,
	"continue": CONTINUE,
}

// LookupIdent returns the keyword Type for `ident` if it's reserved,
// otherwise IDENT. This is how "if" becomes an IF token instead of an
// identifier named "if".
func LookupIdent(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// String gives a human-readable name for a token type — used in error
// messages and in the `pygoc lex` debug command's printed output.
func (t Type) String() string {
	names := map[Type]string{
		ILLEGAL: "ILLEGAL", EOF: "EOF", NEWLINE: "NEWLINE",
		INDENT: "INDENT", DEDENT: "DEDENT",
		IDENT: "IDENT", INT: "INT", FLOAT: "FLOAT", STRING: "STRING",
		AND: "and", OR: "or", NOT: "not", TRUE: "True", FALSE: "False",
		IF: "if", ELIF: "elif", ELSE: "else", WHILE: "while",
		FOR: "for", IN: "in", RANGE: "range", DEF: "def",
		RETURN: "return", BREAK: "break", CONTINUE: "continue",
		PLUS: "+", MINUS: "-", STAR: "*", SLASH: "/", PERCENT: "%",
		DSLASH: "//", ASSIGN: "=", PLUSEQ: "+=", MINUSEQ: "-=",
		STAREQ: "*=", SLASHEQ: "/=", EQ: "==", NEQ: "!=",
		LT: "<", GT: ">", LE: "<=", GE: ">=", ARROW: "->",
		LPAREN: "(", RPAREN: ")", LBRACKET: "[", RBRACKET: "]",
		LBRACE: "{", RBRACE: "}", COMMA: ",", COLON: ":", DOT: ".",
	}
	if n, ok := names[t]; ok {
		return n
	}
	return "UNKNOWN"
}
