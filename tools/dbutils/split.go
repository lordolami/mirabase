package dbutils

import "strings"

// SplitSQLStatements splits a raw SQL batch into individual statements at
// top-level semicolons, honoring single/double quoted strings, backtick and
// bracket identifiers, block comments and line comments. Trailing
// whitespace-only statements are dropped.
//
// The console read path needs this because some engines execute only the
// first statement of a multi-statement query batch through the query API,
// while the inherited provider streams the last statement's result set
// (documented provider difference; native-provider.md).
func SplitSQLStatements(batch string) []string {
	var statements []string

	var b strings.Builder
	flush := func() {
		if strings.TrimSpace(b.String()) != "" {
			statements = append(statements, b.String())
		}
		b.Reset()
	}

	inSingle := false
	inDouble := false
	inBacktick := false
	inBracket := false
	inLineComment := false
	inBlockComment := false

	runes := []rune(batch)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case inLineComment:
			b.WriteRune(r)
			if r == '\n' || r == '\r' {
				inLineComment = false
			}
		case inBlockComment:
			b.WriteRune(r)
			if r == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				b.WriteRune('/')
				i++
				inBlockComment = false
			}
		case inSingle:
			b.WriteRune(r)
			if r == '\'' {
				if i+1 < len(runes) && runes[i+1] == '\'' { // escaped ''
					b.WriteRune('\'')
					i++
				} else {
					inSingle = false
				}
			}
		case inDouble:
			b.WriteRune(r)
			if r == '"' {
				if i+1 < len(runes) && runes[i+1] == '"' { // escaped ""
					b.WriteRune('"')
					i++
				} else {
					inDouble = false
				}
			}
		case inBacktick:
			b.WriteRune(r)
			if r == '`' {
				if i+1 < len(runes) && runes[i+1] == '`' { // escaped ``
					b.WriteRune('`')
					i++
				} else {
					inBacktick = false
				}
			}
		case inBracket:
			b.WriteRune(r)
			if r == ']' {
				if i+1 < len(runes) && runes[i+1] == ']' { // escaped ]]
					b.WriteRune(']')
					i++
				} else {
					inBracket = false
				}
			}
		default:
			switch {
			case r == '-' && i+1 < len(runes) && runes[i+1] == '-':
				inLineComment = true
				b.WriteRune(r)
				b.WriteRune('-')
				i++
			case r == '/' && i+1 < len(runes) && runes[i+1] == '*':
				inBlockComment = true
				b.WriteRune(r)
				b.WriteRune('*')
				i++
			case r == '\'':
				inSingle = true
				b.WriteRune(r)
			case r == '"':
				inDouble = true
				b.WriteRune(r)
			case r == '`':
				inBacktick = true
				b.WriteRune(r)
			case r == '[':
				inBracket = true
				b.WriteRune(r)
			case r == ';':
				flush()
			default:
				b.WriteRune(r)
			}
		}
	}
	flush()

	return statements
}
