# Vector, text, hybrid and fuzzy search

Use `POST /api/mirabase/v1/databases/{id}/collections/{collection}/search` under current target/record authorization. Create the corresponding field/index first. Exact shapes appear in [actual API reference](api-reference.md) and [search source](../../internal/search/search.go).

## Prepare a searchable collection

1. Select an authorized database and a collection whose actual schema exists in that database.
2. Store the text/vector values required by your chosen mode. Embeddings come from your own embedding workflow; MiraBase does not call a vendor to generate them.
3. For full text, create the corresponding search index through `POST /api/mirabase/v1/databases/{id}/collections/{collection}/search-index` with the reviewed fields/tokenizer.
4. Verify readiness and actual matches, then call the search endpoint using the intended actor. An admin-only positive query does not validate an unrelated user's access.
5. Render highlights as escaped data and show incomplete-result state if the response reports it.

## Request examples

These are request bodies, not full application setup. Replace field names and vector dimensions with your real indexed schema; authenticate through your own permitted workflow.

Full text over two text fields:

```json
{
  "mode": "text",
  "text": {
    "fields": ["title", "body"],
    "query": "private database",
    "syntax": "simple",
    "highlight": true
  },
  "limit": 20
}
```

Vector similarity over a compatible three-dimensional field:

```json
{
  "mode": "vector",
  "vector": {
    "field": "embedding",
    "query": [1, 0, 0],
    "metric": "cosine"
  },
  "limit": 10
}
```

Fuzzy matching with a maximum edit distance:

```json
{
  "mode": "fuzzy",
  "field": "title",
  "query": "databse",
  "algorithm": "levenshtein",
  "maximumDistance": 2,
  "limit": 10
}
```

Hybrid requests combine the `vector` and `text` clauses with an `rrf` fusion clause. Check the [request decoder](../../internal/search/search.go) for allowed properties; unknown keys are rejected rather than silently ignored. Fuzzy uses top-level wire fields; do not assume every mode shares identical options.

## Choose a mode

| Mode | Use it for | Important input |
|---|---|---|
| Text | Words/phrases in indexed text | Fields, query syntax, weights and highlights. |
| Vector | Similarity to an embedding | Compatible field, dimension and metric. |
| Hybrid | Combining lexical and semantic retrieval | Both clauses and rank fusion, not raw-score addition. |
| Fuzzy | Near spellings or compatible phonetic matches | Algorithm and distance/similarity threshold. |

Vector mode accepts a field, caller-supplied vector and compatible metric (`cosine|l2|negative_dot|jaccard`, representation-dependent). Dimensions and finite components are checked; undefined cosine on a zero vector is invalid. No embedding-vendor inference call is implied. Exact search is different from a mature ANN product.

Text mode selects fields/query, optional `simple|advanced` syntax, weights and highlights. Tokenizers are `default|en_stem|raw|simple|whitespace|ngram`. Inspect real index readiness/coverage; schema metadata alone is insufficient. Treat highlights as escaped data in a frontend.

Hybrid mode combines vector/text rankings using reciprocal-rank fusion. Native score magnitudes are not directly comparable. Fuzzy matching runs in Go and supports `levenshtein|damerau|osa|hamming|jaro_winkler|soundex` with compatible distance/similarity bounds.

Defaults: 20 results, maximum 100; candidate budget 10,000; five-second deadline. Inspect `complete`, scanned count and elapsed time. Incomplete candidates are not an exact complete top-N guarantee. Stable record-ID ties make order deterministic.

Continuation cursors bind query, target/generation, schema/policy and snapshot; default lease is 60 seconds with 16 active global leases. Continue the identical query. Restart after expiry or incompatible change.

## Handle continuation and failure

Keep the original query and authorized context when continuing. If a lease expires, the schema/generation changes, or the server requests reset, start a new authorized query rather than appending incompatible pages. Respect deadlines and candidate budgets; increasing a result limit does not turn an incomplete scan into an exact answer.

Check index errors separately from no matches. An absent/unready index, an unsupported metric, an invalid zero-vector cosine query and a forbidden target are different outcomes. Never retry a denied ordinary-user query as superuser inside the frontend. The [current product limits](../current-product.md) remain relevant to policy and platform acceptance.

Test overlapping IDs from two private users and hidden fields, rollback/deletes, rebuild interruption and concurrent writes. See [acceptance](../../apis/mirabase_search_acceptance_test.go) and [launch status](../launch/status.md).
