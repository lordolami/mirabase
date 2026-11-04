package pgateway

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PostgreSQL type OIDs emitted in RowDescription. Only types the gateway
// actually reports are defined.
const (
	oidBool        = 16
	oidBytea       = 17
	oidName        = 19
	oidInt8        = 20
	oidInt2        = 21
	oidInt4        = 23
	oidText        = 25
	oidUnknown     = 705
	oidJSON        = 114
	oidJSONB       = 3802
	oidFloat4      = 700
	oidFloat8      = 701
	oidVARCHAR     = 1043
	oidDate        = 1082
	oidTime        = 1083
	oidTimestamp   = 1114
	oidTimestampTZ = 1184
	oidNumeric     = 1700
	oidUUID        = 2950
	oidInet        = 869
	oidCidr        = 650
	oidMacaddr     = 829
	oidMacaddr8    = 774
)

// pgColumn is one RowDescription field.
type pgColumn struct {
	Name   string
	OID    int32
	Format int16 // 0 = text (the only result format this gateway emits)
}

// sqliteTypeToPGType mirrors the pinned engine's PostgreSQL frontend type
// mapping (turso/postgres/server/lib.rs sqlite_type_to_pg_type at the
// pinned revision). Declared column names map identically; anything
// outside the table falls back to TEXT, exactly as the pinned frontend
// does for undetermined columns.
func sqliteTypeToPGType(declared string) int32 {
	upper := strings.ToUpper(strings.TrimSpace(declared))
	switch upper {
	case "INTEGER", "INT", "INT4", "SMALLINT", "INT2", "SERIAL", "SMALLSERIAL":
		return oidInt4
	case "BIGINT", "INT8", "BIGSERIAL":
		return oidInt8
	case "REAL", "FLOAT", "FLOAT4", "FLOAT8", "DOUBLE", "DOUBLE PRECISION", "NUMERIC", "DECIMAL":
		return oidFloat8
	case "TEXT", "VARCHAR", "CHAR", "CHARACTER VARYING", "CHARACTER", "NAME":
		return oidText
	case "BLOB", "BYTEA":
		return oidBytea
	case "BOOLEAN", "BOOL":
		return oidBool
	case "UUID":
		return oidUUID
	case "JSON":
		return oidJSON
	case "JSONB":
		return oidJSONB
	case "DATE":
		return oidDate
	case "TIME", "TIMETZ":
		return oidTime
	case "TIMESTAMP":
		return oidTimestamp
	case "TIMESTAMPTZ":
		return oidTimestampTZ
	case "INET":
		return oidInet
	case "CIDR":
		return oidCidr
	case "MACADDR":
		return oidMacaddr
	case "MACADDR8":
		return oidMacaddr8
	}
	if strings.HasPrefix(upper, "VARCHAR") || strings.HasPrefix(upper, "CHAR") {
		return oidVARCHAR
	}
	if strings.HasPrefix(upper, "NUMERIC") || strings.HasPrefix(upper, "DECIMAL") {
		return oidNumeric
	}
	return oidText
}

// encodeTextValue renders one scanned Go value in PostgreSQL text format.
// nil renders as the NULL marker (length -1); the caller distinguishes it
// before calling this.
func encodeTextValue(v any) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case int64:
		return []byte(strconv.FormatInt(t, 10)), nil
	case int:
		return []byte(strconv.Itoa(t)), nil
	case int32:
		return []byte(strconv.FormatInt(int64(t), 10)), nil
	case float64:
		// Shortest round-trip representation (PG's extra_float_digits=1
		// behavior for FLOAT8 results).
		return []byte(strconv.FormatFloat(t, 'g', -1, 64)), nil
	case float32:
		return []byte(strconv.FormatFloat(float64(t), 'g', -1, 32)), nil
	case bool:
		return []byte(map[bool]string{true: "t", false: "f"}[t]), nil
	case time.Time:
		// The driver parses DATE/TIMESTAMP/DATETIME text into UTC
		// time.Time; re-encode as an explicit UTC timestamp text
		// (type precision is kept explicit: no timezone suffix, the
		// stored instant as-is — the engine stores these values in UTC).
		return []byte(t.UTC().Format("2006-01-02 15:04:05.999999999")), nil
	case []byte:
		// BYTEA text format is \x hex (PostgreSQL bytea_output=hex).
		return []byte(`\x` + fmt.Sprintf("%x", t)), nil
	case string:
		return []byte(t), nil
	default:
		return []byte(fmt.Sprintf("%v", t)), nil
	}
}
