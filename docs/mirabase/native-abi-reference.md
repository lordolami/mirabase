# Native ABI reference

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

MiraBase owns this thin ABI around the pinned engine. It is a specification to implement and qualify, not an upstream API promise. [Native provider](native-provider.md) and [integration contracts](integration-contracts.md) own lifetimes, typed buffers, cancellation, error normalization, capture and packaging. Inputs are copied before return; output buffers have one owner and must be freed once. Operation IDs bind asynchronous completion/cancellation to its owner.


```c
/* MiraBase-owned proposed ABI v1: not an upstream API or implemented library. */
#ifndef MIRABASE_NATIVE_BRIDGE_V1_H
#define MIRABASE_NATIVE_BRIDGE_V1_H
#include <stddef.h>
#include <stdint.h>
typedef uint64_t MbHandle;
typedef struct { uint8_t *data; size_t len; } MbBuffer;
typedef struct { int32_t code; MbBuffer detail_json; } MbStatus;
enum MbFrontend { MB_SQLITE = 1, MB_POSTGRES = 2 };
enum MbTxMode { MB_TX_READ = 1, MB_TX_WAL_WRITE = 2, MB_TX_MVCC_WRITE = 3 };
enum MbCode {
 MB_OK = 0, MB_ROW = 1, MB_DONE = 2, MB_PENDING_IO = 3,
 MB_BUSY = 4, MB_CONFLICT = 5, MB_CANCELLED = 6, MB_CONSTRAINT = 7,
 MB_UNSUPPORTED = 8, MB_CORRUPT = 9, MB_ABI_ERROR = 10, MB_ERROR = 11
};
/* Inputs borrowed for the call and copied before return.
 * Output has one owner; free exactly once. JSON uses owned wire contracts. */
uint32_t mb_abi_version(void);
MbStatus mb_build_manifest(MbBuffer *out);
void mb_buffer_free(MbBuffer value);
MbStatus mb_database_open(const uint8_t *config, size_t len, MbHandle *out);
MbStatus mb_database_close(MbHandle database);
MbStatus mb_connection_open(MbHandle database, uint32_t frontend, MbHandle *out);
MbStatus mb_connection_close(MbHandle connection);
MbStatus mb_statement_prepare(MbHandle connection, const uint8_t *sql, size_t len, MbHandle *out);
MbStatus mb_statement_info(MbHandle statement, MbBuffer *classification_and_columns);
MbStatus mb_statement_bind(MbHandle statement, const uint8_t *typed_values, size_t len);
MbStatus mb_statement_step(MbHandle statement, uint64_t operation_id, MbBuffer *row);
MbStatus mb_statement_close(MbHandle statement);
MbStatus mb_io_poll(uint64_t operation_id, uint32_t timeout_ms);
MbStatus mb_cancel(uint64_t operation_id);
MbStatus mb_transaction_begin(MbHandle connection, uint32_t mode);
MbStatus mb_transaction_commit(MbHandle connection);
MbStatus mb_transaction_rollback(MbHandle connection);
MbStatus mb_capture_configure(MbHandle connection, const uint8_t *mapping, size_t len);
MbStatus mb_capture_read(MbHandle connection, MbBuffer *transaction_changes);
MbStatus mb_pg_session_open(MbHandle database, MbHandle *session);
MbStatus mb_pg_session_state(MbHandle session, MbBuffer *state);
MbStatus mb_pg_session_close(MbHandle session);
MbStatus mb_sync_decode_pipeline(const uint8_t *input, size_t len, MbBuffer *pipeline);
MbStatus mb_sync_encode_pipeline(const uint8_t *result, size_t len, MbBuffer *wire);
MbStatus mb_sync_pull(MbHandle database, const uint8_t *request, size_t len, MbBuffer *wire);
MbStatus mb_sync_checkpoint(MbHandle database, MbBuffer *result);
MbStatus mb_database_quiesce(MbHandle database, MbBuffer *persistent_file_manifest);
MbStatus mb_memory_usage(MbHandle database, uint64_t *allocated_bytes);
MbStatus mb_set_memory_budget(MbHandle database, uint64_t maximum_bytes);
#endif
```
