package pgateway

import "testing"

// TestPGParameterSQL pins the $N -> ?N rewrite: PostgreSQL markers become
// the engine's explicitly numbered markers; literals, dollar-quoted
// strings, quoted identifiers and mid-identifier $ are untouched.
func TestPGParameterSQL(t *testing.T) {
	cases := []struct {
		sql  string
		want string
	}{
		{"SELECT name, qty FROM pgw_ext WHERE qty >= $1",
			"SELECT name, qty FROM pgw_ext WHERE qty >= ?1"},
		{"INSERT INTO pgw_cdc VALUES ($1, $2)",
			"INSERT INTO pgw_cdc VALUES (?1, ?2)"},
		// Out-of-order markers keep their numbers.
		{"SELECT name FROM t WHERE name = $2 AND qty = $1",
			"SELECT name FROM t WHERE name = ?2 AND qty = ?1"},
		// Inside a string literal: untouched.
		{"SELECT 'cost $1 total'", "SELECT 'cost $1 total'"},
		// Dollar-quoted string: untouched.
		{"SELECT $tag$ $1 $tag$", "SELECT $tag$ $1 $tag$"},
		// Mid-identifier $: untouched.
		{"SELECT tbl$1 FROM tbl$1", "SELECT tbl$1 FROM tbl$1"},
		// Quoted identifier: untouched.
		{`SELECT "col$1" FROM t`, `SELECT "col$1" FROM t`},
		// No markers: unchanged.
		{"SELECT 1", "SELECT 1"},
		// Qualified parameter adjacency (a=$1).
		{"SELECT * FROM t WHERE a=$1 AND b=$2", "SELECT * FROM t WHERE a=?1 AND b=?2"},
	}
	for _, tc := range cases {
		sts, err := splitStatements(tc.sql)
		if err != nil {
			t.Fatalf("split %q: %v", tc.sql, err)
		}
		got := pgParameterSQL(tc.sql, sts[0].stripped)
		if got != tc.want {
			t.Errorf("pgParameterSQL(%q) = %q, want %q", tc.sql, got, tc.want)
		}
	}
}

// TestSourceTablesOf pins the table extraction the declared-type resolution
// depends on.
func TestSourceTablesOf(t *testing.T) {
	cases := []struct {
		sql  string
		want []string
	}{
		{"SELECT name, qty FROM pgw_ext", []string{"pgw_ext"}},
		{"SELECT name, qty FROM pgw_ext ORDER BY name", []string{"pgw_ext"}},
		{"SELECT * FROM a JOIN b ON a.x = b.x", []string{"a", "b"}},
		{"SELECT * FROM a, b WHERE a.x = 1", []string{"a", "b"}},
		{"SELECT * FROM a LEFT OUTER JOIN b USING (x)", []string{"a", "b"}},
		{"SELECT x FROM (SELECT y FROM inner_t LIMIT 1) sub", []string{}},
		// A CTE name is a FROM source (its columns resolve to TEXT — the
		// catalog has no declared types for it).
		{"WITH c AS (SELECT 1) SELECT * FROM c", []string{"c"}},
		{"UPDATE t SET x = 1 WHERE y = 2", []string{"t"}},
		{"INSERT INTO t VALUES (1)", []string{"t"}},
		{"DELETE FROM t WHERE x = 1", []string{"t"}},
		{"INSERT INTO s.t VALUES (1)", []string{"t"}},
	}
	for _, tc := range cases {
		got := sourceTablesOf(tc.sql)
		if len(got) != len(tc.want) {
			t.Errorf("sourceTablesOf(%q) = %v, want %v", tc.sql, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("sourceTablesOf(%q) = %v, want %v", tc.sql, got, tc.want)
			}
		}
	}
}
