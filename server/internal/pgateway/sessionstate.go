package pgateway

import (
	"sort"
	"strings"
)

// Gateway session-state control (postgresql-access.md: "Prevent
// cross-session leakage of search_path, settings, temporary state and
// custom session parameters").
//
// The pinned engine's SQL dialect has no SET/RESET/SHOW statements (only
// the engine's own PostgreSQL frontend speaks them, which this gateway does
// not route through), so the gateway implements them ITSELF: settings live
// on the session — never on the engine connection — which means a SET can
// neither leak into the pooled engine connection nor into a later session,
// and SHOW reads back exactly what the session set over the default
// parameter statuses the gateway advertises at startup.
//
// Setting semantics (PostgreSQL):
//   - SET [SESSION] name = value / TO value: session scope.
//   - SET LOCAL: transaction-block scope only (elsewhere refused, 25P01),
//     cleared when the block ends.
//   - RESET name / RESET ALL.
//   - SHOW name / SHOW ALL.
//   - Physically fixed parameters (server_version, server_encoding, ...)
//     cannot be set (22023); client_encoding is pinned to UTF8.

// baseParameterStatuses are the parameter statuses the gateway advertises
// at startup and the defaults SHOW reports; the runtime-injected values
// (session_authorization, application_name, mirabase_*) are added by the
// startup handshake.
var baseParameterStatuses = [][2]string{
	{"server_version", "16.0"},
	{"server_encoding", "UTF8"},
	{"client_encoding", "UTF8"},
	{"DateStyle", "ISO, MDY"},
	{"integer_datetimes", "on"},
	{"standard_conforming_strings", "on"},
	{"TimeZone", "UTC"},
	{"in_hot_standby", "off"},
}

// fixedParameters are reported by the gateway but physically fixed: setting
// them would make the wire lie about behavior that does not change.
var fixedParameters = map[string]bool{
	"server_version":      true,
	"server_encoding":     true,
	"integer_datetimes":   true,
	"in_hot_standby":      true,
	"standard_conforming_strings": true,
	"mirabase_application":        true,
	"mirabase_database":           true,
	"session_authorization":       true,
}

// reportableParameters are re-announced with a ParameterStatus message when
// a successful SET changes them (PostgreSQL announces reportable GUCs);
// keys are the lowercased setting names.
var reportableParameters = map[string]bool{
	"timezone":         true,
	"application_name": true,
}

// parameterStatusKey is the canonical wire spelling of a reportable
// parameter name (the wire keys keep PostgreSQL's capitalization).
func parameterStatusKey(name string) string {
	if strings.EqualFold(name, "timezone") {
		return "TimeZone"
	}
	return name
}

// sessStmt is one parsed SET/RESET/SHOW statement.
type sessStmt struct {
	kind  string // "set" | "reset" | "show"
	all   bool
	local bool
	name  string // lowercased parameter name ("" for SHOW ALL/RESET ALL)
	value string // raw value text (SET only)
}

// parseSessionStatement parses one SET/RESET/SHOW statement from the
// ORIGINAL statement text (values may be quoted literals, which the
// stripped copy blanks). Unknown shapes are syntax errors.
func parseSessionStatement(text string) (sessStmt, *PGError) {
	rest := strings.TrimSpace(text)
	upper := strings.ToUpper(rest)
	var ss sessStmt
	switch {
	case strings.HasPrefix(upper, "SET "):
		ss.kind = "set"
		rest = rest[4:]
	case strings.HasPrefix(upper, "RESET "):
		ss.kind = "reset"
		rest = rest[6:]
	case strings.HasPrefix(upper, "SHOW "):
		ss.kind = "show"
		rest = rest[5:]
	case upper == "RESET":
		return sessStmt{kind: "reset", all: true}, nil
	case upper == "SHOW":
		return sessStmt{kind: "show", all: true}, nil
	default:
		return ss, newPGError(stateSyntaxError, "unrecognized session-state statement")
	}
	rest = strings.TrimSpace(rest)
	upper = strings.ToUpper(rest)

	// Scope and TIME ZONE spellings.
	if ss.kind == "set" {
		if strings.HasPrefix(upper, "LOCAL ") {
			ss.local = true
			rest = strings.TrimSpace(rest[6:])
			upper = strings.ToUpper(rest)
		} else if strings.HasPrefix(upper, "SESSION ") {
			rest = strings.TrimSpace(rest[8:])
			upper = strings.ToUpper(rest)
		}
		if strings.HasPrefix(upper, "TIME ZONE") {
			// SET TIME ZONE <value> sets the TimeZone parameter.
			rest = strings.TrimSpace(rest[len("TIME ZONE"):])
			ss.name = "TimeZone"
			ss.value = unquoteSettingValue(rest)
			if ss.value == "" {
				return ss, newPGError(stateSyntaxError, "SET TIME ZONE requires a value")
			}
			return ss, nil
		}
	}

	// The parameter name (or ALL).
	end := 0
	for end < len(rest) {
		c := rest[end]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '=' {
			break
		}
		end++
	}
	name := strings.TrimSpace(rest[:end])
	rest = rest[end:]
	if ss.kind != "set" {
		// RESET/SHOW: the name is everything left (no = value).
		name = strings.TrimSpace(name + rest)
		rest = ""
	}
	if strings.EqualFold(strings.TrimSpace(name), "ALL") {
		ss.all = true
		name = ""
	} else if !validSettingName(name) {
		return ss, newPGErrorf(stateSyntaxError, "invalid parameter name in %s", strings.ToUpper(ss.kind))
	}
	ss.name = strings.ToLower(name)
	if ss.kind == "set" {
		// The value: "=" or TO, then the remainder.
		trimmed := strings.TrimLeft(rest, " \t\n\r")
		if strings.HasPrefix(trimmed, "=") {
			rest = trimmed[1:]
		} else if len(trimmed) >= 2 && strings.EqualFold(trimmed[:2], "TO") &&
			(len(trimmed) == 2 || trimmed[2] == ' ' || trimmed[2] == '\t' || trimmed[2] == '\n' || trimmed[2] == '\r') {
			rest = trimmed[2:]
		} else {
			return ss, newPGError(stateSyntaxError, "SET requires a value assignment (= or TO)")
		}
		ss.value = unquoteSettingValue(rest)
		if ss.value == "" {
			return ss, newPGError(stateSyntaxError, "SET requires a value assignment (= or TO)")
		}
	}
	return ss, nil
}

// validSettingName accepts the PostgreSQL parameter-name shape (letters,
// digits, underscores, dots for custom parameters).
func validSettingName(name string) bool {
	if name == "" {
		return false
	}
	for i, c := range name {
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		case c == '.':
		default:
			return false
		}
	}
	return len(name) <= 128
}

// unquoteSettingValue trims whitespace and one layer of surrounding quotes
// from a SET value; the raw text is kept otherwise.
func unquoteSettingValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"') {
			inner := v[1 : len(v)-1]
			inner = strings.ReplaceAll(inner, "''", "'")
			inner = strings.ReplaceAll(inner, `""`, `"`)
			return inner
		}
	}
	if len(v) > 1024 {
		v = v[:1024]
	}
	return v
}

// executeSessionStatement applies one SET/RESET/SHOW statement to the
// session's own state and writes the wire responses (CommandComplete, or
// RowDescription+DataRows for SHOW). Nothing here touches the engine
// connection.
func (s *session) executeSessionStatement(st pgStatement) (execResult, error) {
	res := execResult{}
	ss, perr := parseSessionStatement(st.text)
	if perr != nil {
		return res, perr
	}
	switch ss.kind {
	case "set":
		if perr := s.applySet(ss); perr != nil {
			return res, perr
		}
		res.tag = "SET"
	case "reset":
		if perr := s.applyReset(ss); perr != nil {
			return res, perr
		}
		res.tag = "RESET"
	case "show":
		cols, rows, perr := s.showResult(ss)
		if perr != nil {
			return res, perr
		}
		if err := s.writeRowDescription(cols); err != nil {
			return res, err
		}
		for _, row := range rows {
			if err := s.writeDataRow(row); err != nil {
				return res, err
			}
		}
		res.rowsReturned = len(rows)
		res.tag = "SHOW"
	}
	if err := s.writeCommandComplete(res.tag); err != nil {
		return res, err
	}
	return res, nil
}

// applySet applies one SET to the session state.
func (s *session) applySet(ss sessStmt) *PGError {
	if fixedParameters[ss.name] {
		return newPGErrorf(stateInvalidParameterValue, "parameter %q cannot be changed", ss.name)
	}
	if ss.name == "client_encoding" {
		if !strings.EqualFold(ss.value, "UTF8") && !strings.EqualFold(ss.value, "UNICODE") {
			return newPGErrorf(stateInvalidParameterValue,
				"encoding %q is not supported; the gateway speaks UTF8", ss.value)
		}
		return nil // already UTF8; nothing to store
	}
	if ss.local && !s.inTx {
		return newPGError(stateNoActiveSQLTransaction,
			"SET LOCAL can only be used in transaction blocks")
	}
	if s.settings == nil {
		s.settings = map[string]string{}
	}
	if s.txSettings == nil {
		s.txSettings = map[string]string{}
	}
	if ss.local {
		s.txSettings[ss.name] = ss.value
	} else {
		s.settings[ss.name] = ss.value
		// Reportable GUCs are re-announced (ParameterStatus) immediately.
		if reportableParameters[ss.name] {
			if err := s.writeParameterStatusMsg(parameterStatusKey(ss.name), ss.value); err != nil {
				return newPGError(stateInternalError, err.Error())
			}
		}
	}
	return nil
}

// applyReset applies one RESET (name or ALL) to the session state.
func (s *session) applyReset(ss sessStmt) *PGError {
	if !ss.all {
		if fixedParameters[ss.name] {
			return newPGErrorf(stateInvalidParameterValue, "parameter %q cannot be changed", ss.name)
		}
		delete(s.settings, ss.name)
		delete(s.txSettings, ss.name)
		return nil
	}
	s.settings = map[string]string{}
	s.txSettings = map[string]string{}
	return nil
}

// showResult renders a SHOW statement's shape and rows: one column named
// after the parameter (SHOW name), or the name/setting/description triple
// (SHOW ALL). Values come from the transaction scope (inside a block),
// then the session scope, then the advertised defaults.
func (s *session) showResult(ss sessStmt) ([]pgColumn, [][]any, *PGError) {
	if ss.all {
		values := map[string]string{}
		for _, kv := range baseParameterStatuses {
			values[strings.ToLower(kv[0])] = kv[1]
		}
		for k, v := range s.settings {
			values[k] = v
		}
		if s.inTx {
			for k, v := range s.txSettings {
				values[k] = v
			}
		}
		if s.cred != nil {
			values["session_authorization"] = s.cred.ID
		}
		names := make([]string, 0, len(values))
		for k := range values {
			names = append(names, k)
		}
		sort.Strings(names)
		cols := []pgColumn{
			{Name: "name", OID: oidText},
			{Name: "setting", OID: oidText},
			{Name: "description", OID: oidText},
		}
		rows := make([][]any, 0, len(names))
		for _, n := range names {
			rows = append(rows, []any{n, values[n], ""})
		}
		return cols, rows, nil
	}
	v, ok := s.settingValue(ss.name)
	if !ok {
		return nil, nil, newPGErrorf(stateUndefinedObject, "unrecognized configuration parameter %q", ss.name)
	}
	return []pgColumn{{Name: parameterStatusKey(ss.name), OID: oidText}}, [][]any{{v}}, nil
}

// settingValue resolves one parameter's current value for the session.
func (s *session) settingValue(name string) (string, bool) {
	if name == "session_authorization" && s.cred != nil {
		return s.cred.ID, true
	}
	if s.inTx {
		if v, ok := s.txSettings[name]; ok {
			return v, true
		}
	}
	if v, ok := s.settings[name]; ok {
		return v, true
	}
	for _, kv := range baseParameterStatuses {
		if strings.EqualFold(kv[0], name) {
			return kv[1], true
		}
	}
	if name == "application_name" {
		return "", true
	}
	return "", false
}

// endTransactionBlock clears the transaction-scoped settings (SET LOCAL
// reverts at COMMIT and ROLLBACK) and the failed-block flag bookkeeping the
// callers already perform.
func (s *session) endTransactionBlock() {
	s.txSettings = map[string]string{}
}

// writeParameterStatusMsg writes one ParameterStatus under the write mutex.
func (s *session) writeParameterStatusMsg(key, value string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeParameterStatus(s.w(), key, value)
}
