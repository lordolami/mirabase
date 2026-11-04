# Fields

Collections are composed of typed fields. Common field families include text,
number, boolean, email, URL, date, select, file, relation, JSON, editor,
password, and autodate fields.

Field definitions control:

- whether a value is required;
- whether multiple values are allowed;
- minimum, maximum, pattern, and option constraints;
- whether a field is hidden from ordinary API responses;
- relation target and cascade behavior;
- file limits and thumbnail settings;
- generated creation/update timestamps for autodate fields.

Hidden fields are not ordinary client output. Passwords and authentication
secrets receive additional protection and are not valid search, live-query, or
reference-copy projection fields.

Schema changes are versioned. Existing records and indexes must be migrated or
explicitly reported as incompatible; a failed schema transition must not leave
a silently truncated collection.

## Added managed-schema adapters

The current owned registry contains `int64`, `decimal`, `uuid`, `typed-date`, `typed-time`, `typed-timestamp`, `network-address` and `vector`. Inspect [fieldkinds.go](../../internal/application/fieldkinds.go) and the schema plan for actual storage/validation. These adapters do not prove every dashboard/core-field/SDK combination is complete. STRUCT/UNION/DOMAIN/custom adapters are not established merely because native SQL accepts a type. [Current inventory](../current-product.md) and [full type contracts](../mirabase/collections-and-types.md) preserve the integration obligations.
