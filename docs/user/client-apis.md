# Client APIs

MiraBase serves two API families:

- the inherited record API under `/api/collections/...`, `/api/files`, and
  `/api/realtime`, kept stable for existing clients;
- the MiraBase API under `/api/mirabase/v1/...` for databases, applications,
  live queries, search, sync, reference copies, imports, and operator surfaces.

## SDK compatibility

Existing JavaScript and Dart clients written against the inherited record API
keep working against MiraBase servers. The dashboard's API preview shows
copy-paste examples in that client style; the class and package names in those
examples are compatibility aliases preserved by the product contract, not a
second product identity.

MiraBase-specific capabilities (live queries, hybrid/fuzzy search, reference
copies, the PostgreSQL gateway) are used through direct HTTP or the MiraBase
extensions; see the operation reference in the product documentation set.

## Errors

Read actual status and envelope. Inherited errors use `{status,message,data}`;
additive handlers can add `data.mirabase` codes/context. Not every handler
currently supplies every planned structured field. Common codes where supplied:
`invalid_request` (400), `unauthorized`/`forbidden` (401/403), `not_found`
(404), `cursor_expired` (410), `reference_stale` and `resource_exhausted`
(503, retryable where marked). Branch on actual status/code, not the human
message. See [error handling](errors-and-troubleshooting.md).
