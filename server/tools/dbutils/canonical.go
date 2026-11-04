package dbutils

import "strings"

// CanonicalDDLExpr normalizes a stored SQL default-expression text for
// provider-insensitive comparison: whitespace is removed entirely and a
// single balanced pair of outer parentheses is stripped while it wraps the
// whole expression. Engines that canonicalize stored DDL (e.g. the pinned
// Turso engine) re-render expressions with different spacing and wrapping
// (native-provider.md documents the delta; "inherited introspection
// assertions use exact provider-specific canonical text").
func CanonicalDDLExpr(s string) string {
	s = removeWhitespace(s)
	for len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' && balancedWhole(s) {
		s = s[1 : len(s)-1]
	}
	return s
}

// CanonicalSQLKeywordForm normalizes stored SQL DDL text for
// provider-insensitive comparison: case-insensitive keywords and spacing
// are collapsed while identifier tokens keep their separators.
func CanonicalSQLKeywordForm(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			space = true
		case r == '(' || r == ')' || r == ',':
			// no space adjacent to punctuation
			space = false
			b.WriteRune(r)
		default:
			if space && b.Len() > 0 {
				prev := rune(b.String()[b.Len()-1])
				if prev != '(' && prev != ',' {
					b.WriteRune(' ')
				}
			}
			space = false
			b.WriteRune(r)
		}
	}
	return b.String()
}

func removeWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// balancedWhole reports whether the first '(' of s matches the last ')'.
func balancedWhole(s string) bool {
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(s)-1 {
				return false
			}
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}
