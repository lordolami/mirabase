# MiraBase changelog

## Unreleased

- Added live query subscriptions with durable replay, keyed deltas, policy
  injection, cancellation, and schema/generation resets.
- Completed reference-copy policies with scheduled refresh, bounded projection
  transactions, quarantine, sequence protection, authorized reads, and
  `reference_stale` reporting.
- Added hybrid search with reciprocal-rank fusion and fuzzy search with
  Levenshtein, Damerau, OSA, Hamming, Jaro-Winkler, and Soundex algorithms.
- Added the opt-in PostgreSQL wire gateway with SCRAM authentication, scoped
  credentials, extended query protocol support, native-DML CDC, and safe
  statement enforcement.
- Replaced upstream product-documentation destinations with the owned MiraBase
  documentation set and in-product help surfaces.
- Replaced upstream security and contribution documents with MiraBase-owned
  policies.

## Foundation

MiraBase v0.1 is built from the pinned PocketBase v0.41.1 source reference and
extended with the Turso engine and MiraBase-owned services. Upstream copyright
and license notices are preserved in `LICENSE.md`; upstream release history is
not republished as MiraBase release notes.
