package mbengine

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	turso_libs "github.com/tursodatabase/turso-go-platform-libs"
	tursogo "turso.tech/database/tursogo"
)

// MbHandle mirrors `typedef uint64_t MbHandle;`.
type MbHandle = uint64

// MbBuffer mirrors `typedef struct { uint8_t *data; size_t len; } MbBuffer;`.
type MbBuffer struct {
	Data *uint8
	Len  uintptr
}

// MbStatus mirrors `typedef struct { int32_t code; MbBuffer detail_json; } MbStatus;`
// (int32 at offset 0, the buffer naturally aligned at offset 8, size 24).
type MbStatus struct {
	Code   int32
	Detail MbBuffer
}

// MbCode values (native-abi-reference.md).
const (
	MbOK          = 0
	MbRow         = 1
	MbDone        = 2
	MbPendingIO   = 3
	MbBusy        = 4
	MbConflict    = 5
	MbCancelled   = 6
	MbConstraint  = 7
	MbUnsupported = 8
	MbCorrupt     = 9
	MbAbiError    = 10
	MbError       = 11
)

// MbFrontend / MbTxMode values.
const (
	MbFrontendSQLITE = 1

	MbTxRead      = 1
	MbTxWalWrite  = 2
	MbTxMvccWrite = 3
)

// The Go binding binds only the *_into out-pointer variants (status through
// an explicit pointer): portable across every purego target, unlike
// by-value struct returns (supported natively by purego only on darwin).
// The canonical by-value mb_* symbols remain the documented C ABI surface.

var c_mb_abi_version func() uint32

var c_mb_bridge_init_into func(outStatus *MbStatus)
var c_mb_build_manifest_into func(outStatus *MbStatus, out *MbBuffer)
var c_mb_database_open_into func(outStatus *MbStatus, config *uint8, len uintptr, out *MbHandle)
var c_mb_database_close_into func(outStatus *MbStatus, database MbHandle)
var c_mb_connection_open_into func(outStatus *MbStatus, database MbHandle, frontend uint32, out *MbHandle)
var c_mb_connection_close_into func(outStatus *MbStatus, connection MbHandle)
var c_mb_statement_prepare_into func(outStatus *MbStatus, connection MbHandle, sql *uint8, len uintptr, out *MbHandle)
var c_mb_statement_info_into func(outStatus *MbStatus, statement MbHandle, out *MbBuffer)
var c_mb_statement_bind_into func(outStatus *MbStatus, statement MbHandle, typedValues *uint8, len uintptr)
var c_mb_statement_step_into func(outStatus *MbStatus, statement MbHandle, operationID uint64, row *MbBuffer)
var c_mb_statement_close_into func(outStatus *MbStatus, statement MbHandle)
var c_mb_io_poll_into func(outStatus *MbStatus, operationID uint64, timeoutMS uint32)
var c_mb_cancel_into func(outStatus *MbStatus, operationID uint64)
var c_mb_transaction_begin_into func(outStatus *MbStatus, connection MbHandle, mode uint32)
var c_mb_transaction_commit_into func(outStatus *MbStatus, connection MbHandle)
var c_mb_transaction_rollback_into func(outStatus *MbStatus, connection MbHandle)
var c_mb_database_quiesce_into func(outStatus *MbStatus, database MbHandle, persistentFileManifest *MbBuffer)
var c_mb_memory_usage_into func(outStatus *MbStatus, database MbHandle, allocatedBytes *uint64)
var c_mb_set_memory_budget_into func(outStatus *MbStatus, database MbHandle, maximumBytes uint64)
var c_mb_buffer_free_into func(value *MbBuffer)

// ErrFatal reports a startup probe failure; the application must refuse to
// continue (fatal before application files open).
var ErrFatal = errors.New("native engine startup check failed")

var (
	ensureOnce sync.Once
	ensureErr  error
	ensureDone bool
)

// Ensure performs the full payload trust + ABI + capability startup
// sequence exactly once. It must run before any database connection is
// opened. Errors are fatal for the caller (bootstrap refuses to continue).
func Ensure() error {
	ensureOnce.Do(func() {
		ensureErr = ensure()
		ensureDone = ensureErr == nil
	})
	return ensureErr
}

// Ready reports whether Ensure completed successfully.
func Ready() bool {
	return ensureDone
}

func ensure() error {
	// 1. Extract + load the embedded payload through the owned loader. The
	// loader verifies the embedded bytes and the extracted cache copy
	// against its link-time digest flag.
	handle, err := turso_libs.LoadTursoLibrary(turso_libs.LoadTursoLibraryConfig{})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFatal, err)
	}
	_ = handle

	// 2. Independent verification with the mbengine-side pin: cross-check
	// the two link-time flags and re-hash the loaded file.
	if err := Verify(); err != nil {
		return fmt.Errorf("%w: %v", ErrFatal, err)
	}

	// 3. Bind the owned ABI symbols (out-pointer variants).
	bindMB(handle)

	// 4. Bind the driver's library-load Once to our verified strategy so
	// the pristine driver's first Open cannot trigger a different load.
	tursogo.InitLibrary(turso_libs.LoadTursoLibraryConfig{})

	// 5. Engine global setup + ABI version.
	if st := mbBridgeInit(); st.Code != MbOK {
		return fmt.Errorf("%w: bridge init failed: %s", ErrFatal, statusDetail(st))
	}
	if v := c_mb_abi_version(); v != 1 {
		return fmt.Errorf("%w: unexpected ABI version %d (want 1)", ErrFatal, v)
	}

	// 6. Build manifest: engine revision and library digest must match the
	// pin exactly.
	var manifest MbBuffer
	if st := mbBuildManifest(&manifest); st.Code != MbOK {
		return fmt.Errorf("%w: build manifest failed: %s", ErrFatal, statusDetail(st))
	}
	manifestJSON := bufferBytes(manifest)
	if err := checkManifest(manifestJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrFatal, err)
	}

	// 7. Read-only capability probes in an isolated scratch database.
	if err := probeCapabilities(); err != nil {
		return fmt.Errorf("%w: %v", ErrFatal, err)
	}
	return nil
}

func bindMB(handle uintptr) {
	purego.RegisterLibFunc(&c_mb_abi_version, handle, "mb_abi_version")
	purego.RegisterLibFunc(&c_mb_bridge_init_into, handle, "mb_bridge_init_into")
	purego.RegisterLibFunc(&c_mb_build_manifest_into, handle, "mb_build_manifest_into")
	purego.RegisterLibFunc(&c_mb_database_open_into, handle, "mb_database_open_into")
	purego.RegisterLibFunc(&c_mb_database_close_into, handle, "mb_database_close_into")
	purego.RegisterLibFunc(&c_mb_connection_open_into, handle, "mb_connection_open_into")
	purego.RegisterLibFunc(&c_mb_connection_close_into, handle, "mb_connection_close_into")
	purego.RegisterLibFunc(&c_mb_statement_prepare_into, handle, "mb_statement_prepare_into")
	purego.RegisterLibFunc(&c_mb_statement_info_into, handle, "mb_statement_info_into")
	purego.RegisterLibFunc(&c_mb_statement_bind_into, handle, "mb_statement_bind_into")
	purego.RegisterLibFunc(&c_mb_statement_step_into, handle, "mb_statement_step_into")
	purego.RegisterLibFunc(&c_mb_statement_close_into, handle, "mb_statement_close_into")
	purego.RegisterLibFunc(&c_mb_io_poll_into, handle, "mb_io_poll_into")
	purego.RegisterLibFunc(&c_mb_cancel_into, handle, "mb_cancel_into")
	purego.RegisterLibFunc(&c_mb_transaction_begin_into, handle, "mb_transaction_begin_into")
	purego.RegisterLibFunc(&c_mb_transaction_commit_into, handle, "mb_transaction_commit_into")
	purego.RegisterLibFunc(&c_mb_transaction_rollback_into, handle, "mb_transaction_rollback_into")
	purego.RegisterLibFunc(&c_mb_database_quiesce_into, handle, "mb_database_quiesce_into")
	purego.RegisterLibFunc(&c_mb_memory_usage_into, handle, "mb_memory_usage_into")
	purego.RegisterLibFunc(&c_mb_set_memory_budget_into, handle, "mb_set_memory_budget_into")
	purego.RegisterLibFunc(&c_mb_buffer_free_into, handle, "mb_buffer_free_into")
}

// Go-side wrappers (status in, err-like handling by callers).

func mbBridgeInit() MbStatus {
	var st MbStatus
	c_mb_bridge_init_into(&st)
	return st
}

func mbBuildManifest(out *MbBuffer) MbStatus {
	var st MbStatus
	c_mb_build_manifest_into(&st, out)
	return st
}

func mbDatabaseOpen(config *uint8, n uintptr, out *MbHandle) MbStatus {
	var st MbStatus
	c_mb_database_open_into(&st, config, n, out)
	return st
}

func mbDatabaseClose(database MbHandle) MbStatus {
	var st MbStatus
	c_mb_database_close_into(&st, database)
	return st
}

func mbConnectionOpen(database MbHandle, frontend uint32, out *MbHandle) MbStatus {
	var st MbStatus
	c_mb_connection_open_into(&st, database, frontend, out)
	return st
}

func mbConnectionClose(connection MbHandle) MbStatus {
	var st MbStatus
	c_mb_connection_close_into(&st, connection)
	return st
}

func mbStatementPrepare(connection MbHandle, sql *uint8, n uintptr, out *MbHandle) MbStatus {
	var st MbStatus
	c_mb_statement_prepare_into(&st, connection, sql, n, out)
	return st
}

func mbStatementInfo(statement MbHandle, out *MbBuffer) MbStatus {
	var st MbStatus
	c_mb_statement_info_into(&st, statement, out)
	return st
}

func mbStatementBind(statement MbHandle, typedValues *uint8, n uintptr) MbStatus {
	var st MbStatus
	c_mb_statement_bind_into(&st, statement, typedValues, n)
	return st
}

func mbStatementStep(statement MbHandle, operationID uint64, row *MbBuffer) MbStatus {
	var st MbStatus
	c_mb_statement_step_into(&st, statement, operationID, row)
	return st
}

func mbStatementClose(statement MbHandle) MbStatus {
	var st MbStatus
	c_mb_statement_close_into(&st, statement)
	return st
}

func mbIoPoll(operationID uint64, timeoutMS uint32) MbStatus {
	var st MbStatus
	c_mb_io_poll_into(&st, operationID, timeoutMS)
	return st
}

func mbCancel(operationID uint64) MbStatus {
	var st MbStatus
	c_mb_cancel_into(&st, operationID)
	return st
}

func mbTransactionBegin(connection MbHandle, mode uint32) MbStatus {
	var st MbStatus
	c_mb_transaction_begin_into(&st, connection, mode)
	return st
}

func mbTransactionCommit(connection MbHandle) MbStatus {
	var st MbStatus
	c_mb_transaction_commit_into(&st, connection)
	return st
}

func mbTransactionRollback(connection MbHandle) MbStatus {
	var st MbStatus
	c_mb_transaction_rollback_into(&st, connection)
	return st
}

func mbDatabaseQuiesce(database MbHandle, out *MbBuffer) MbStatus {
	var st MbStatus
	c_mb_database_quiesce_into(&st, database, out)
	return st
}

func mbMemoryUsage(database MbHandle, out *uint64) MbStatus {
	var st MbStatus
	c_mb_memory_usage_into(&st, database, out)
	return st
}

func mbSetMemoryBudget(database MbHandle, maximumBytes uint64) MbStatus {
	var st MbStatus
	c_mb_set_memory_budget_into(&st, database, maximumBytes)
	return st
}

// mbBufferFree releases an owned MbBuffer exactly once.
func mbBufferFree(b MbBuffer) {
	c_mb_buffer_free_into(&b)
}

// bufferBytes copies an owned MbBuffer into Go memory and frees the buffer.
func bufferBytes(b MbBuffer) []byte {
	if b.Data == nil || b.Len == 0 {
		c_mb_buffer_free_into(&b)
		return nil
	}
	out := make([]byte, b.Len)
	copy(out, unsafe.Slice(b.Data, b.Len))
	c_mb_buffer_free_into(&b)
	return out
}

// statusDetail extracts the human-readable part of a failure status.
func statusDetail(st MbStatus) string {
	b := bufferBytes(st.Detail)
	if len(b) == 0 {
		return fmt.Sprintf("code=%d", st.Code)
	}
	return fmt.Sprintf("code=%d detail=%s", st.Code, string(b))
}
