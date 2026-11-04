// Package pgateway implements the MiraBase PostgreSQL wire gateway
// (postgresql-access.md): a TLS-only pgwire listener with SCRAM-SHA-256
// authentication against the server-issued scoped credentials, session
// isolation and the simple query protocol executed on the pinned engine
// through the shared database handles.
//
// Authority model: the application and database context are resolved from
// the startup packet BEFORE authentication; the credential is then verified
// with a real RFC 5802 exchange and must carry purpose "pg", match the
// resolved database and the live generation/policy epoch. An application
// superuser is never consulted and never gains instance-operator authority
// through this surface: whole-database SQL authority comes exclusively
// from the credential's sql.read/sql.write scopes.
package pgateway

import (
	"fmt"
	"io"
	"strings"
)

// SQLSTATE codes used by the gateway (PostgreSQL error codes; the pinned
// engine's frontend reports the same classes for engine errors).
const (
	stateInternalError            = "XX000"
	stateProtocolViolation        = "08P01"
	stateConnectionException      = "08006"
	stateFeatureNotSupported      = "0A000"
	stateActiveSQLTransaction     = "25001"
	stateNoActiveSQLTransaction   = "25P01"
	stateInFailedSQLTransaction   = "25P02"
	stateInvalidPassword          = "28P01"
	stateInvalidAuthSpecification = "28000"
	stateInvalidCatalogName       = "3D000"
	stateUndefinedTable           = "42P01"
	stateUndefinedObject          = "42704"
	stateUndefinedColumn          = "42703"
	stateUndefinedFunction        = "42883"
	stateDuplicateObject          = "42710"
	stateInsufficientPrivilege    = "42501"
	stateSyntaxError              = "42601"
	stateQueryCanceled            = "57014"
	stateCannotConnectNow         = "57P03"
	stateTooManyConnections       = "53300"
	stateUniqueViolation          = "23505"
	stateForeignKeyViolation      = "23503"
	stateNotNullViolation         = "23502"
	stateCheckViolation           = "23514"
	stateDivisionByZero           = "22012"
	stateUndefinedStatement       = "26000"
	stateInvalidParameterValue    = "22023"
)

// PGError is one wire-reportable PostgreSQL error.
type PGError struct {
	Severity string
	Code     string
	Message  string
	Detail   string
	Hint     string
}

// Error implements error (the message only; the SQLSTATE travels in the
// ErrorResponse fields).
func (e *PGError) Error() string { return e.Message }

// newPGError builds an ERROR-severity error.
func newPGError(code, message string) *PGError {
	return &PGError{Severity: "ERROR", Code: code, Message: message}
}

// newPGErrorf builds an ERROR-severity error from a format.
func newPGErrorf(code, format string, args ...any) *PGError {
	return newPGError(code, fmt.Sprintf(format, args...))
}

// errPrivilege is the parsed sql.read/sql.write enforcement error
// (postgresql-access.md: "sql.read denies writes/unsafe PRAGMAs/extension/
// file access through parsed native enforcement").
func errPrivilege(format string, args ...any) *PGError {
	return newPGError(stateInsufficientPrivilege, fmt.Sprintf(format, args...))
}

// errProtocol is a wire-protocol violation.
func errProtocol(format string, args ...any) *PGError {
	return newPGError(stateProtocolViolation, fmt.Sprintf(format, args...))
}

// errAuth is a credential-level authentication/authorization failure.
// Responses do not distinguish unknown from unauthorized credentials
// beyond the standard password/authorization split (28P01 for a failed
// exchange, 28000 for a refused binding).
func errAuth(code, format string, args ...any) *PGError {
	return newPGError(code, fmt.Sprintf(format, args...))
}

// errManagedDDL refuses managed-object DDL while the reconciliation
// adapter does not exist, directing the operator to the schema planner
// (postgresql-access.md: "Until a statement has that reconciliation
// adapter, reject it for managed objects and direct the operator to the
// schema planner").
func errManagedDDL(name string) *PGError {
	e := newPGError(stateFeatureNotSupported,
		fmt.Sprintf("DDL on managed collection %q is not available through the PostgreSQL gateway; use the schema planner (schema.plan/schema.apply)", name))
	e.Hint = "Collection schema changes run through the controlled administrative workflow."
	return e
}

// writeError sends one ErrorResponse. The writer is the session's framed
// writer; a write failure is surfaced to the caller (the connection is
// unusable afterwards).
func writeError(w io.Writer, e *PGError) error {
	return writeErrorNotice(w, 'E', e)
}

// writeNotice sends one NoticeResponse: the same field layout as an
// ErrorResponse with the NoticeResponse message type. Notices carry
// non-fatal information — a statement degrades (or succeeds where
// PostgreSQL only warns) and the client keeps the session
// (postgresql-access.md wire qualification: "errors/notices").
func writeNotice(w io.Writer, e *PGError) error {
	if e.Severity == "" {
		e.Severity = "WARNING"
	}
	if e.Severity == "ERROR" {
		e.Severity = "WARNING"
	}
	return writeErrorNotice(w, 'N', e)
}

// writeErrorNotice renders the shared ErrorResponse/NoticeResponse payload.
func writeErrorNotice(w io.Writer, msgType byte, e *PGError) error {
	var b []byte
	appendErrorField := func(code byte, value string) {
		b = append(b, code)
		b = append(b, value...)
		b = append(b, 0)
	}
	if e.Severity == "" {
		e.Severity = "ERROR"
		if msgType == 'N' {
			e.Severity = "WARNING"
		}
	}
	appendErrorField('S', e.Severity)
	appendErrorField('V', e.Severity)
	appendErrorField('C', e.Code)
	appendErrorField('M', strings.ReplaceAll(e.Message, "\x00", ""))
	if e.Detail != "" {
		appendErrorField('D', strings.ReplaceAll(e.Detail, "\x00", ""))
	}
	if e.Hint != "" {
		appendErrorField('H', strings.ReplaceAll(e.Hint, "\x00", ""))
	}
	b = append(b, 0)
	_, err := writeMessage(w, msgType, b)
	return err
}
