# Collections, fields and value formats

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Managed cloud](../cloud/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Inherited model

Keep base, auth and view collections and every inherited field configuration. Existing fields retain their established JSON and SQL behavior. Do not change number into a decimal string or JSON into an opaque blob during a provider swap. Schema import/export includes new field settings, capability requirements and version; older clients receive readable unsupported-field errors rather than data corruption.

New fields register through core.Fields and implement Field validation, SQL ColumnType, PrepareValue, value getters/setters, driver conversion and applicable record interceptors. Both dashboard and SDK generators derive from the same metadata. A native SQL type without these adapters is not a completed field feature.

## Added field contracts

| Type | HTTP/SDK value | Validation and storage |
|---|---|---|
| vector | Array of finite numbers; sparse form {dimensions,indices,values} | Fixed dimensions/encoding; sorted unique sparse indices; no NaN/Infinity; packed native representation |
| int64 | Canonical signed decimal string | Range -9223372036854775808..9223372036854775807; no floating conversion |
| decimal | Canonical decimal string | Declared precision/scale; reject overflow and unintended rounding |
| uuid | Canonical lowercase UUID string | Version restriction optional; native UUID encoding; never replace record ID implicitly |
| array | JSON nested arrays of declared element type | Dimension/length/nullability constraints; homogeneous elements; no ragged matrix |
| binary | Base64 string through the dedicated typed envelope | Byte limit; native bytea/blob; record file field remains separate |
| typed-date/time/timestamp | Canonical ISO forms specified in schema | Strict parsing; micro/nanosecond precision metadata; distinguish instant from local timestamp |
| network-address | Canonical string | Validate IPv4/IPv6/network prefix/MAC rather than trusting textual upstream alias |
| struct | JSON object with declared members | Reject unknown members unless explicitly allowed; member nullability/defaults |
| union | {tag,value} | Exactly one declared variant; validate payload using that variant |
| domain/custom | Underlying public value plus declared type ID | Versioned encoder/decoder/coercion and domain checks |

Default vector dimension maximum is 4096, sparse nonzero maximum 4096; schema may lower them. An increase requires measured native/HTTP limits and an explicit configured bound. Arrays default to at most 4 dimensions and 10000 total elements. Binary field default limit is 1 MiB. These are tunable safety defaults, not engine mathematical maxima.

JSON wire typed values in administrative SQL/CDC use tagged envelopes: {type:"int64",value:"..."}, {type:"blob",base64:"..."}, {type:"decimal",value:"..."} and equivalent declared composites. Ordinary record fields expose the column's documented direct JSON value. NULL, missing field, empty string and an empty array are distinct.

## Defaults and generation

Preserve inherited required/default/modifier behavior. New schema default is one of literal, uuid4, uuid7, sequence or an approved deterministic expression with declared volatility. SQL expressions are superuser schema configuration; ordinary clients cannot submit an expression as a value. Record forms evaluate defaults once inside the selected transaction and return the final stored value.

Sequences have qualified names, integer bounds/increment/cycle and declared frontend/session semantics. A sequence gap is permitted and documented; never promise gapless external identifiers. UUID generation uses native cryptographic randomness where required and tests collisions/format/version. Timestamps specify UTC/local semantics; timestamptz does not prove preservation of input offset.

## STRICT schemas and compatibility

Native types may require STRICT tables. Do not convert every inherited table in place as a side effect. A collection declares its storage schema requirements; converting an existing collection is a planned rebuild with preflight value checks, indexes/triggers/files preserved, restart recovery and rollback copy.

New custom/domain/composite definitions are revisioned and referenced by stable type ID. Prevent deleting a type with live fields/views/index dependencies. Encoders must be total for valid values; invalid existing rows block migration with IDs/counts, not coercion or deletion. Composite indexing restrictions are visible in the editor.

## Field behavior integration

Required, hidden, system, unique, min/max, mutable modifiers, search/filter comparisons, sorting, projection, relation expansion, API preview, hooks, import/export, realtime, backups, CDC and both sync paths each need a type test. Passwords, token keys and protected auth metadata are never emitted by generic serialization. Type-aware validation errors retain field paths, including array indices/struct members, and do not expose native internals.

Source-of-truth contracts contain typeId, wire format, storage format, validation, comparison/index support, defaults and version. No generic "any" field stands in for these adapters.
