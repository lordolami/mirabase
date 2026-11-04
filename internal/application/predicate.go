package application

// Shared rule-compilation seam: Inherited filter rules compile to a
// parameterized SQL predicate through the inherited, parsed filter
// machinery (tools/search + the core field resolver) — never interpolated
// (queries-and-search.md: "Never interpolate client field names, filter
// strings ... into executable SQL"). Consumers: live-plan policy
// injection (the actor's current rules), reference-copy record selection
// (the policy's selection AST) and the search surface's list/view rules.

import (
	"fmt"
	"regexp"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"mirabase/core"
	"mirabase/tools/search"
)

// paramMarkerRe matches the dbx parameter markers ({:name}) the compiled
// expression carries.
var paramMarkerRe = regexp.MustCompile(`\{:([^}]+)\}`)

// Expressions retain DBX identifier markers until a DBX query is created.
// These consumers execute through database/sql, so finish identifier quoting
// here using the same builder instead of sending [[column]] to the engine.
var predicateIdentRe = regexp.MustCompile(`(\{\{[\w\-\. ]+\}\}|\[\[[\w\-\. ]+\]\])`)

// multiMatchAliasRe matches the resolver's @request.auth relation-join
// aliases; a fragment that references them needs the resolver's join
// machinery, which the parameterized-predicate surfaces do not carry
// (those rules fail closed instead of silently unfiltered).
var multiMatchAliasRe = regexp.MustCompile(`__mm_`)

// BuildRulePredicate compiles the rule against the collection with the
// request context (nil info is allowed for system-side rules that never
// reference @request) and returns the SQL predicate with ? placeholders
// and positional arguments. The predicate references the collection's
// table name exactly as the resolver renders it, so it binds to the same
// table on a target database connection.
func BuildRulePredicate(base *core.BaseApp, coll *core.Collection, rule string, info *core.RequestInfo) (string, []any, error) {
	if strings.TrimSpace(rule) == "" {
		return "", nil, nil
	}
	resolver := core.NewRecordFieldResolver(base, coll, info, false)
	expr, err := search.FilterData(rule).BuildExpr(resolver)
	if err != nil {
		return "", nil, fmt.Errorf("invalid_request: the rule does not compile against the collection: %v", err)
	}
	params := dbx.Params{}
	builder := dbx.NewFromDB(base.DBConn(), "sqlite3")
	fragment := expr.Build(builder, params)
	fragment = predicateIdentRe.ReplaceAllStringFunc(fragment, func(marker string) string {
		if strings.HasPrefix(marker, "{{") {
			return builder.QuoteTableName(marker[2 : len(marker)-2])
		}
		return builder.QuoteColumnName(marker[2 : len(marker)-2])
	})
	if multiMatchAliasRe.MatchString(fragment) {
		return "", nil, fmt.Errorf("invalid_request: the rule requires a relation join against the request identity, which is not available on this surface")
	}
	args := make([]any, 0, len(params))
	sqlText := paramMarkerRe.ReplaceAllStringFunc(fragment, func(marker string) string {
		key := marker[2 : len(marker)-1]
		args = append(args, params[key])
		return "?"
	})
	return sqlText, args, nil
}
