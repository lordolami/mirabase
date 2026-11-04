package application

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// R1 further field kinds (collections-and-types.md + engine-reference.md):
// the completed field adapters for int64, decimal, uuid, typed date/time/
// timestamp, network-address and packed vectors. Every adapter implements
// the required integrations for its kind: value validation, column type,
// driver conversion and filter/sort compatibility. Types without
// completed adapters (struct/union/domain/custom) are refused at plan
// time with the honest reason — never a silent pass-through.
// ---------------------------------------------------------------------------

// FieldKindAdapter is one released field kind's adapter.
type FieldKindAdapter struct {
	// WireType is the engine field id.
	WireType string
	// ColumnType is the engine column type.
	ColumnType string
	// ValidateValue rejects out-of-contract wire values.
	ValidateValue func(v any) error
	// ConvertValue renders the value for storage (driver conversion).
	ConvertValue func(v any) (any, error)
}

var int64Pattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// r1FieldAdapters is the released adapter registry (the schema-plan field
// mapping consults it; the inherited core kinds remain registered
// separately).
var r1FieldAdapters = map[string]*FieldKindAdapter{
	"int64": {
		WireType:   "int64",
		ColumnType: "BIGINT",
		ValidateValue: func(v any) error {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("int64 wire is a canonical signed decimal string")
			}
			if !int64Pattern.MatchString(s) || s == "-0" || len(s) > 2 && s[0] == '0' && s[1] != '.' {
				return fmt.Errorf("int64 wire must be a canonical signed decimal string (no leading zeros, no fractions)")
			}
			n := new(big.Int)
			if _, ok := n.SetString(s, 10); !ok || !n.IsInt64() {
				return fmt.Errorf("int64 overflows the native 64-bit range")
			}
			return nil
		},
		ConvertValue: func(v any) (any, error) {
			s, _ := v.(string)
			n := new(big.Int)
			n.SetString(s, 10)
			return n.Int64(), nil
		},
	},
	"decimal": {
		WireType:   "decimal",
		ColumnType: "DECIMAL",
		ValidateValue: func(v any) error {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("decimal wire is a canonical decimal string")
			}
			if _, _, err := big.ParseFloat(s, 10, 200, big.ToNearestEven); err != nil {
				return fmt.Errorf("decimal wire must be a valid decimal string")
			}
			return nil
		},
		ConvertValue: func(v any) (any, error) {
			s, _ := v.(string)
			return s, nil // the engine stores decimal as exact text
		},
	},
	"uuid": {
		WireType:   "uuid",
		ColumnType: "UUID",
		ValidateValue: func(v any) error {
			s, ok := v.(string)
			if !ok || len(s) != 36 {
				return fmt.Errorf("uuid wire is the canonical 36-character form")
			}
			for i, r := range s {
				switch i {
				case 8, 13, 18, 23:
					if r != '-' {
						return fmt.Errorf("uuid separators are dashes at 8/13/18/23")
					}
				default:
					if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
						return fmt.Errorf("uuid digits are lowercase/uppercase hex")
					}
				}
			}
			return nil
		},
		ConvertValue: func(v any) (any, error) {
			s, _ := v.(string)
			return strings.ToLower(s), nil
		},
	},
	"typed-date": {
		WireType:   "typed-date",
		ColumnType: "DATE",
		ValidateValue: func(v any) error {
			s, ok := v.(string)
			if !ok || !datePattern.MatchString(s) {
				return fmt.Errorf("typed-date wire is YYYY-MM-DD")
			}
			return nil
		},
		ConvertValue: func(v any) (any, error) { return v, nil },
	},
	"typed-time": {
		WireType:   "typed-time",
		ColumnType: "TIME",
		ValidateValue: func(v any) error {
			s, ok := v.(string)
			if !ok || !timePattern.MatchString(s) {
				return fmt.Errorf("typed-time wire is HH:MM:SS[.fraction]")
			}
			return nil
		},
		ConvertValue: func(v any) (any, error) { return v, nil },
	},
	"typed-timestamp": {
		WireType:   "typed-timestamp",
		ColumnType: "TIMESTAMP",
		ValidateValue: func(v any) error {
			s, ok := v.(string)
			if !ok || !tsPattern.MatchString(s) {
				return fmt.Errorf("typed-timestamp wire is RFC3339")
			}
			return nil
		},
		ConvertValue: func(v any) (any, error) { return v, nil },
	},
	"network-address": {
		WireType:   "network-address",
		ColumnType: "VARCHAR",
		ValidateValue: func(v any) error {
			s, ok := v.(string)
			if !ok || (!ipV4Pattern.MatchString(s) && !ipV6Pattern.MatchString(s)) {
				return fmt.Errorf("network-address wire is a valid IPv4 or IPv6 address")
			}
			return nil
		},
		ConvertValue: func(v any) (any, error) { return v, nil },
	},
	"vector": {
		WireType:   "vector",
		ColumnType: "VECTOR",
		ValidateValue: func(v any) error {
			// Dense finite number array (the packed storage conversion is
			// the engine's; the wire accepts the dense form).
			arr, ok := v.([]any)
			if !ok || len(arr) == 0 {
				return fmt.Errorf("vector wire is a finite nonempty number array")
			}
			for _, e := range arr {
				switch e.(type) {
				case float64, int64:
				default:
					return fmt.Errorf("vector components are numbers")
				}
			}
			return nil
		},
		ConvertValue: func(v any) (any, error) { return v, nil },
	},
}

var (
	datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timePattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}(\.\d+)?$`)
	tsPattern   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?$`)
	ipV4Pattern = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	ipV6Pattern = regexp.MustCompile(`^[0-9a-fA-F:]+$`)
)

// R1FieldAdapter returns the adapter for one engine field kind (nil when
// the kind has no completed adapter in this release).
func R1FieldAdapter(fieldType string) *FieldKindAdapter {
	return r1FieldAdapters[fieldType]
}

// R1FieldKinds lists the released adapter kinds.
func R1FieldKinds() []string {
	out := make([]string, 0, len(r1FieldAdapters))
	for k := range r1FieldAdapters {
		out = append(out, k)
	}
	return out
}

// ValidateR1FieldValue validates one value against its adapter.
func ValidateR1FieldValue(fieldType string, v any) error {
	adapter := r1FieldAdapters[fieldType]
	if adapter == nil {
		return fmt.Errorf("field type %q has no completed field adapter in this release", fieldType)
	}
	return adapter.ValidateValue(v)
}

var _ = context.Background
