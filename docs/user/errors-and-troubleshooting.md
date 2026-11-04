# Errors and troubleshooting

Read the actual HTTP status and structured response. Inherited errors use `{status,message,data}`; additive handlers can include `data.mirabase` details. Do not assume every handler currently supplies every planned error field. Keep a redacted request/job ID, operation and target generation for diagnosis.

| Condition | What to do |
|---|---|
| Unauthenticated/expired session | Authenticate using your own account; do not substitute a superuser session in a frontend. |
| Forbidden/invisible target | Recheck current membership/grant/rules and selector. A 404 can intentionally hide existence. |
| Schema/generation/policy mismatch | Fetch actual current metadata, preserve the original intent and replan/rebootstrap as required. |
| Conflict/idempotency conflict | Reconcile the original receipt and request body; never regenerate IDs to disguise a retry. |
| Cursor expiry/reset | Fetch a fresh authorized snapshot; don't concatenate pages or patches from incompatible boundaries. |
| Maintenance/resource exhaustion | Respect retry guidance and bounded backoff; inspect jobs/queues/disk/connection use. |
| Unknown commit/lost response | Inspect durable receipt/job/target state before replaying. Missing history is not proof of rollback. |
| Storage/key/archive failure | Preserve live and recovery copies, stop destructive retries, check hashes/keys/coverage and the failed job phase. |

## Startup problems

Missing digest pins mean the executable was built with the wrong command. Use `go run ./tools/release`; don't bypass the native checks or copy an arbitrary DLL into PATH. A digest mismatch means the payload/build pin differs; preserve the evidence and replace/rebuild from the trusted source. A sidecar checksum cannot authorize changed code.

Missing target payload means that platform is not packaged yet. A successful Go cross-compile with no usable native payload is not a working server. Use the correct artifact and consult [launch status](../launch/status.md).

An explicitly enabled PostgreSQL gateway with missing/invalid TLS material prevents successful serve startup. Check `MIRABASE_POSTGRES_ENABLED`, `MIRABASE_POSTGRES_LISTEN`, certificate/key settings and file permissions. Do not enable plaintext as a workaround.

If the dashboard loads but data operations fail, verify backend URL/origins, current auth, collection rules, target selector, lifecycle state and actual schema revision. Test one normal record request before adding search/sync/live workflows.

## Recovery boundary

Stop competing writers, preserve complete DB/sidecars/files/key references, inspect the failed job/journal and validate an isolated restore. Never delete an unknown directory because a cached registry didn't list it, or replace a missing/corrupt DB with an empty one. See [operations](operations.md) and [launch procedure](../launch/README.md).
