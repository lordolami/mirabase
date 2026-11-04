# Queries, vector search and text search

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Existing queries

Keep inherited filter grammar, parameter binding, sorting, pagination, expand and field projection. New functions register through the search token/function extension seam and a shared typed query planner. Never interpolate client field names, filter strings, tokenizers, sort directions or SQL expressions into executable SQL.

Each query resolves one database, published schema revision, actor and rules. Apply list/view rules and field protection before results leave the server. Query caches include context ID, generation, schema revision and policy epoch; never cache one user's authorized response for another.

## Search request

POST /api/mirabase/v1/databases/{id}/collections/{collection}/search accepts a discriminated mode:

- vector: field, query vector, metric, limit, optional filter and projection.
- text: fields, query, syntax=simple or advanced, limit, optional filter, weights and highlight request.
- hybrid: vector and text clauses, filter, limit and fusion={kind:"rrf",k:60}.
- fuzzy: field, query, algorithm, maximumDistance or minimumSimilarity, limit and filter.

Limit defaults to 20 and is 1..100; candidate budget defaults to 10000 scanned matching records and request deadline to 5 seconds. Operators can tune bounded budgets. Exhaustion returns an explicit incomplete/error result, never a deceptively complete nearest-neighbor list. Exact vector mode produces the exact top results within the authorized/filter domain; it cannot prelimit an unauthorized global candidate set and leak it.

Response: databaseId, generation, schemaRevision, items (records plus score and optional authorized highlights), mode, metric, complete, scannedCount, elapsedMs and optional nextCursor. Scores have mode-specific meaning; RRF combines ranks rather than incomparable raw magnitudes. Stable ties use record ID. Cursor binds plan hash, schema/generation/policy epoch, snapshot lease and last sort position. Send it as cursor on the next search request with the identical query. Default snapshot lease is 60 seconds and the global maximum is 16 active search cursor leases. Expiry returns cursor_expired; do not silently continue against a different snapshot. Every continuation rechecks policy and can require a new search.

## Vector semantics

Validate dimensions, encoding, finite components and metric compatibility. Zero vectors with undefined cosine produce a field error. Sparse indices must be sorted/unique/in-range. Distance direction is explicit: smaller L2/cosine-distance/negative-dot is closer; returned score object names the quantity. Jaccard applies only to the qualified representation.

Native SQL implements exact distance where available; no model inference or external embedding service is required. A vector field stores embeddings supplied by the application. Adding an embedding provider would be an independent extension with cost/privacy configuration, not a hidden network call.

Approximate indexing is not claimed complete from the toy IVF source. The current required vector contract is exact; a future ANN feature needs recall, filter correctness, update/delete and failure qualification before advertisement.

## Text and fuzzy semantics

FTS indexes declare indexed fields, tokenizer/version, language, JSON paths and weights. Index creation/rebuild is a durable job with old index serving or explicit unavailable status. Commits update the native index coherently. Uncommitted text is not guaranteed searchable by the upstream path; surface commit visibility. Deletes remove entries after commit and stale results are rechecked against current records/rules.

Advanced query syntax is opt-in, parsed and bounded. Boolean, phrases, prefixes and field scopes follow the qualified native syntax. Malformed queries return validation errors. Highlight fragments are escaped/sanitized data; never emit untrusted HTML directly. Unsupported native snippets are not shown as available.

Fuzzy algorithms declare distance versus similarity and their allowed thresholds. Expensive cross-record comparisons need prefilters and budgets. Digest/regex/time/network helpers are explicit typed filter functions; regex evaluation and expression depth are bounded against denial of service.

## Other SQL features

Administrative SQL supports tested arrays, domains, composites, sequences, UUID, JSON, math/time, aggregate/window/ordered-set and richer joins/CTEs. View schema validation checks frontend compatibility. SQLite and PostgreSQL differences remain in the capability matrix, including recursive CTE discrepancies.

Qualification compares authorized search with a brute-force reference, uses overlapping user data and ties, mutates records while rebuilding, exercises rollback/crash, malformed vectors/text/regex, mixed Unicode and pagination invalidation. Performance evidence records data size, dimensions, selectivity, CPU, cache state and engine build.

