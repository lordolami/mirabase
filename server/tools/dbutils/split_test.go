package dbutils

import (
	"reflect"
	"testing"
)

func TestSplitSQLStatements(t *testing.T) {
	scenarios := []struct {
		name  string
		batch string
		want  []string
	}{
		{
			"single statement",
			"select 1",
			[]string{"select 1"},
		},
		{
			"two statements",
			"select 1;select 2",
			[]string{"select 1", "select 2"},
		},
		{
			"trailing semicolon",
			"select 1;",
			[]string{"select 1"},
		},
		{
			"empty",
			"  ;  ;",
			nil,
		},
		{
			"semicolon inside string literal",
			`select 'a;b' as x; select 2`,
			[]string{`select 'a;b' as x`, " select 2"},
		},
		{
			"escaped quote inside string",
			`select 'it''s;fine'; select 2`,
			[]string{`select 'it''s;fine'`, " select 2"},
		},
		{
			"semicolon inside double-quoted identifier",
			`select "a;b" as x; select 2`,
			[]string{`select "a;b" as x`, " select 2"},
		},
		{
			"semicolon inside backtick identifier",
			"select `a;b` as x; select 2",
			[]string{"select `a;b` as x", " select 2"},
		},
		{
			"semicolon inside bracket identifier",
			"select [a;b] as x; select 2",
			[]string{"select [a;b] as x", " select 2"},
		},
		{
			"semicolon inside line comment",
			"select 1 -- comment; still comment\n; select 2",
			[]string{"select 1 -- comment; still comment\n", " select 2"},
		},
		{
			"semicolon inside block comment",
			"select 1 /* comment; still */; select 2",
			[]string{"select 1 /* comment; still */", " select 2"},
		},
		{
			"unterminated string does not split",
			"select 'a;b",
			[]string{"select 'a;b"},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got := SplitSQLStatements(s.batch)
			if !reflect.DeepEqual(got, s.want) {
				t.Fatalf("got %q, want %q", got, s.want)
			}
		})
	}
}
