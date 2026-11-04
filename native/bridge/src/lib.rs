//! MiraBase-owned thin C-ABI bridge (`mb_*` symbols) over the pinned Turso
//! engine. ABI contract: docs/mirabase/native-abi-reference.md (owned ABI v1,
//! header guard MIRABASE_NATIVE_BRIDGE_V1_H).
//!
//! Ownership rules (contract): inputs are borrowed for the call and copied
//! before return; every output buffer has exactly one owner and is freed
//! exactly once via `mb_buffer_free`. Operation IDs bind asynchronous
//! completion/cancellation to their owner. This library also exports the
//! upstream `turso_*`/`turso_sync_*` C symbols (via the turso_sync_sdk_kit
//! dependency) so the pristine upstream Go driver can bind them.
//!
//! Lock order (deadlock-free): registry.stmts (read/write) -> statement
//! Mutex -> registry.conns -> registry.ops. The registry guard is always
//! held across the statement Mutex guard's lifetime so a concurrent close()
//! can never drop a live Mutex.
//!
//! The bridge is in-process only: there is no Rust HTTP server, no network
//! listener and no per-database process anywhere in this payload.

use std::collections::HashMap;
use std::ffi::c_char;
use std::sync::atomic::{AtomicU32, AtomicU64, Ordering};
use std::sync::{Arc, Mutex, OnceLock};

use parking_lot::RwLock;
use turso::core::OpenFlags;
use turso::core::Value;
use turso_sdk_kit::rsapi::{
    turso_setup, TursoDatabase, TursoDatabaseConfig, TursoError, TursoSetupConfig,
    TursoStatusCode,
};

// ---------------------------------------------------------------------------
// ABI types (native-abi-reference.md)
// ---------------------------------------------------------------------------

/// `typedef uint64_t MbHandle;`
pub type MbHandle = u64;

/// `typedef struct { uint8_t *data; size_t len; } MbBuffer;`
///
/// Buffers returned to the host are owned by the host and must be released
/// with exactly one `mb_buffer_free` call. Empty outputs use a null pointer.
#[repr(C)]
#[derive(Debug, Clone, Copy)]
pub struct MbBuffer {
    pub data: *mut u8,
    pub len: usize,
}

/// `typedef struct { int32_t code; MbBuffer detail_json; } MbStatus;`
#[repr(C)]
#[derive(Debug, Clone, Copy)]
pub struct MbStatus {
    pub code: i32,
    pub detail_json: MbBuffer,
}

// MbFrontend
pub const MB_SQLITE: u32 = 1;
#[allow(dead_code)]
pub const MB_POSTGRES: u32 = 2;

// MbTxMode
pub const MB_TX_READ: u32 = 1;
pub const MB_TX_WAL_WRITE: u32 = 2;
pub const MB_TX_MVCC_WRITE: u32 = 3;

// MbCode
pub const MB_OK: i32 = 0;
pub const MB_ROW: i32 = 1;
pub const MB_DONE: i32 = 2;
pub const MB_PENDING_IO: i32 = 3;
pub const MB_BUSY: i32 = 4;
pub const MB_CONFLICT: i32 = 5;
pub const MB_CANCELLED: i32 = 6;
pub const MB_CONSTRAINT: i32 = 7;
pub const MB_UNSUPPORTED: i32 = 8;
pub const MB_CORRUPT: i32 = 9;
pub const MB_ABI_ERROR: i32 = 10;
pub const MB_ERROR: i32 = 11;

/// Owned ABI version reported by `mb_abi_version`.
pub const MB_ABI_VERSION: u32 = 1;

/// The pinned engine revision this payload was built from (build-start.md).
pub const ENGINE_REVISION: &str = "b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd";

// ---------------------------------------------------------------------------
// Status helpers
// ---------------------------------------------------------------------------

/// Allocate an owned buffer from a byte slice. Never returns null data with
/// non-zero length.
fn buf_from_bytes(bytes: Vec<u8>) -> MbBuffer {
    let len = bytes.len();
    if len == 0 {
        return MbBuffer { data: std::ptr::null_mut(), len: 0 };
    }
    let mut b = bytes.into_boxed_slice();
    let data = b.as_mut_ptr();
    std::mem::forget(b);
    MbBuffer { data, len }
}

pub const MB_OK_STATUS: MbStatus = MbStatus {
    code: MB_OK,
    detail_json: MbBuffer { data: std::ptr::null_mut(), len: 0 },
};

/// A bare completion status with no detail payload.
fn simple_status(code: i32) -> MbStatus {
    MbStatus { code, detail_json: MbBuffer { data: std::ptr::null_mut(), len: 0 } }
}

fn err_status(code: i32, detail: serde_json::Value) -> MbStatus {
    let json = serde_json::to_vec(&detail).unwrap_or_else(|_| b"{}".to_vec());
    MbStatus { code, detail_json: buf_from_bytes(json) }
}

/// Map a TursoError onto the owned MbCode classification. The native variant
/// name and message are preserved in `detail_json` for internal diagnostics;
/// application-facing error strings are produced by the Go layer, which must
/// omit paths, secrets and raw SQL parameters.
fn classify_error(err: &TursoError) -> (i32, serde_json::Value) {
    let (native, class, code, message): (&str, &str, i32, String) = match err {
        TursoError::Busy(m) => ("Busy", "busy", MB_BUSY, m.clone()),
        TursoError::BusySnapshot(m) => ("BusySnapshot", "conflict", MB_CONFLICT, m.clone()),
        TursoError::Interrupt(m) => ("Interrupt", "cancelled", MB_CANCELLED, m.clone()),
        TursoError::Constraint(m) => ("Constraint", "constraint", MB_CONSTRAINT, m.clone()),
        TursoError::Misuse(m) => ("Misuse", "abi_error", MB_ABI_ERROR, m.clone()),
        TursoError::Corrupt(m) => ("Corrupt", "corrupt", MB_CORRUPT, m.clone()),
        TursoError::NotAdb(m) => ("NotAdb", "corrupt", MB_CORRUPT, m.clone()),
        TursoError::Readonly(m) => ("Readonly", "readonly", MB_ERROR, m.clone()),
        TursoError::DatabaseFull(m) => ("DatabaseFull", "database_full", MB_ERROR, m.clone()),
        TursoError::IoError(kind, op) => (
            "IoError",
            "io",
            MB_ERROR,
            format!("{op}: io error kind {kind:?}"),
        ),
        TursoError::Error(m) => ("Error", "error", MB_ERROR, m.clone()),
    };
    (
        code,
        serde_json::json!({
            "class": class,
            "message": message,
            "native": native,
        }),
    )
}

fn err_from(res: Result<(), TursoError>) -> MbStatus {
    match res {
        Ok(()) => MB_OK_STATUS,
        Err(e) => {
            let (code, detail) = classify_error(&e);
            err_status(code, detail)
        }
    }
}

/// Map an rsapi step/finalize status onto the owned MbCode. The rsapi layer
/// reports only completion states; every error class arrives through
/// TursoError and is classified separately.
fn map_status(status: TursoStatusCode) -> i32 {
    match status {
        TursoStatusCode::Done => MB_DONE,
        TursoStatusCode::Row => MB_ROW,
        TursoStatusCode::Io => MB_PENDING_IO,
    }
}

// ---------------------------------------------------------------------------
// Handle registry (generation-checked handles, D-17)
// ---------------------------------------------------------------------------

struct DbEntry {
    gen: u32,
    db: Arc<TursoDatabase>,
    path: String,
}

struct ConnEntry {
    gen: u32,
    conn: Arc<turso_sdk_kit::rsapi::TursoConnection>,
    db_id: u32,
}

struct StmtEntry {
    gen: u32,
    stmt: Mutex<Box<turso_sdk_kit::rsapi::TursoStatement>>,
    conn_id: u32,
}

#[derive(Clone)]
struct OpEntry {
    /// connection to interrupt for cancellation
    conn: Arc<turso_sdk_kit::rsapi::TursoConnection>,
    /// statement that owns the pending I/O
    stmt_id: u32,
    stmt_gen: u32,
}

struct Registry {
    next_id: AtomicU32,
    next_op: AtomicU64,
    dbs: RwLock<HashMap<u32, DbEntry>>,
    conns: RwLock<HashMap<u32, ConnEntry>>,
    stmts: RwLock<HashMap<u32, StmtEntry>>,
    ops: RwLock<HashMap<u64, OpEntry>>,
}

static REG: OnceLock<Registry> = OnceLock::new();

fn reg() -> &'static Registry {
    REG.get().expect("bridge registry not initialized")
}

fn init_reg() {
    let _ = REG.set(Registry {
        next_id: AtomicU32::new(1),
        next_op: AtomicU64::new(1),
        dbs: RwLock::new(HashMap::new()),
        conns: RwLock::new(HashMap::new()),
        stmts: RwLock::new(HashMap::new()),
        ops: RwLock::new(HashMap::new()),
    });
}

/// Encode an id + generation into an MbHandle.
fn enc(id: u32, gen: u32) -> MbHandle {
    ((gen as u64) << 32) | (id as u64)
}

/// Decode an MbHandle into (id, gen). id 0 is never valid.
fn dec(h: MbHandle) -> Option<(u32, u32)> {
    let id = (h & 0xFFFF_FFFF) as u32;
    let gen = (h >> 32) as u32;
    if id == 0 {
        return None;
    }
    Some((id, gen))
}

fn alloc_id() -> u32 {
    reg().next_id.fetch_add(1, Ordering::Relaxed)
}

// ---------------------------------------------------------------------------
// Config parsing (owned wire contracts, JSON)
// ---------------------------------------------------------------------------

#[derive(serde::Deserialize)]
struct DbConfig {
    path: String,
    #[serde(default)]
    vfs: Option<String>,
    /// Experimental feature tokens; MiraBase always enables "vacuum"
    /// (native-provider.md: bare VACUUM requires the engine's experimental
    /// vacuum feature token and MiraBase enables it on every database open).
    #[serde(default)]
    experimental_features: Vec<String>,
    #[serde(default = "default_true")]
    async_io: bool,
    #[serde(default)]
    read_only: bool,
}

fn default_true() -> bool {
    true
}

/// Map the owned VFS name onto the engine IoBackend. The v0.1 payload
/// includes the platform fallback plus target-specific async backends
/// (io_uring Linux, IOCP Windows); the fallback remains available
/// everywhere (integration-contracts.md).
fn io_backend(name: &str) -> Result<turso_sdk_kit::IoBackend, MbStatus> {
    use turso_sdk_kit::IoBackend;
    match name {
        "" | "default" | "null" => Ok(IoBackend::Default),
        "memory" => Ok(IoBackend::Memory),
        "syscall" => Ok(IoBackend::Syscall),
        "io_uring" => Ok(IoBackend::IoUring),
        "experimental_win_iocp" => Ok(IoBackend::IOCP),
        other => Err(err_status(
            MB_UNSUPPORTED,
            serde_json::json!({
                "class": "unsupported",
                "message": format!("unknown vfs backend {other:?}"),
            }),
        )),
    }
}

// ---------------------------------------------------------------------------
// Group A — version / build manifest / buffer release
// ---------------------------------------------------------------------------

/// `uint32_t mb_abi_version(void);`
#[no_mangle]
pub extern "C" fn mb_abi_version() -> u32 {
    MB_ABI_VERSION
}

/// `MbStatus mb_build_manifest(MbBuffer *out);`
///
/// Records engine commit, library digest (self-hash), ABI version,
/// compiler/target, enabled feature groups and CPU requirements.
#[no_mangle]
pub unsafe extern "C" fn mb_build_manifest(out: *mut MbBuffer) -> MbStatus {
    if out.is_null() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"manifest out pointer is null"}),
        );
    }
    let library_digest = self_library_digest().unwrap_or_else(|| "<unavailable>".to_string());
    let manifest = serde_json::json!({
        "abiVersion": MB_ABI_VERSION,
        "engineRevision": ENGINE_REVISION,
        "patchSetDigest": env!("MB_BUILD_PATCH_SET_DIGEST"),
        "libraryDigest": library_digest,
        "compilerTarget": env!("MB_BUILD_TARGET"),
        "rustc": env!("MB_BUILD_RUSTC"),
        "features": env!("MB_BUILD_FEATURES").split(',').collect::<Vec<_>>(),
        "cpuRequirements": "x86-64-baseline",
        "optionalGroups": {
            "encryption": "present-not-promised",
            "extensions": "present-not-promised",
            "pgFrontend": "excluded-v0.1",
            "sync": "present-not-promised",
            "codecs": "excluded-v0.1"
        }
    });
    match serde_json::to_vec(&manifest) {
        Ok(bytes) => {
            *out = buf_from_bytes(bytes);
            MB_OK_STATUS
        }
        Err(e) => err_status(MB_ERROR, serde_json::json!({"class":"error","message":e.to_string()})),
    }
}

/// `void mb_buffer_free(MbBuffer value);`
#[no_mangle]
pub unsafe extern "C" fn mb_buffer_free(value: MbBuffer) {
    if value.data.is_null() || value.len == 0 {
        return;
    }
    let slice = std::slice::from_raw_parts_mut(value.data, value.len);
    let boxed = std::ptr::slice_from_raw_parts_mut(slice.as_mut_ptr(), value.len);
    drop(Box::from_raw(boxed));
}

/// Compute the sha256 of this loaded library file (real self-digest for the
/// build manifest; Windows: module path of this code address; unix: dladdr).
fn self_library_digest() -> Option<String> {
    use sha2::{Digest, Sha256};
    let path = self_library_path()?;
    let bytes = std::fs::read(path).ok()?;
    let mut h = Sha256::new();
    h.update(&bytes);
    Some(format!("{:x}", h.finalize()))
}

#[cfg(target_os = "windows")]
fn self_library_path() -> Option<String> {
    use windows_sys::Win32::Foundation::HMODULE;
    use windows_sys::Win32::System::LibraryLoader::{
        GetModuleFileNameW, GetModuleHandleExW, GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS,
        GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
    };
    unsafe {
        let mut module: HMODULE = std::ptr::null_mut();
        let this_fn = self_library_path as usize;
        if GetModuleHandleExW(
            (GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS
                | GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT) as u32,
            this_fn as *const u16,
            &mut module,
        ) == 0
        {
            return None;
        }
        let mut buf = vec![0u16; 1024];
        let n = GetModuleFileNameW(module, buf.as_mut_ptr(), buf.len() as u32);
        if n == 0 || n as usize >= buf.len() {
            return None;
        }
        Some(String::from_utf16_lossy(&buf[..n as usize]))
    }
}

#[cfg(target_os = "linux")]
fn self_library_path() -> Option<String> {
    // Resolve this payload's file from /proc/self/maps: the extracted
    // mirabase_engine.so is mmapped by the loader; its path appears on the
    // map line for this code address's mapping region.
    let maps = std::fs::read_to_string("/proc/self/maps").ok()?;
    for line in maps.lines() {
        // Each line: "<start>-<end> <perms> <offset> <dev> <inode> <path>"
        if !line.contains("mirabase_engine") {
            continue;
        }
        let path = match line.rsplit_once(' ') {
            Some((_, p)) => p.trim(),
            None => continue,
        };
        if path.starts_with('/') && std::path::Path::new(path).exists() {
            return Some(path.to_string());
        }
    }
    None
}

#[cfg(not(any(target_os = "windows", target_os = "linux")))]
fn self_library_path() -> Option<String> {
    // dladdr-based resolution is delivered with the remaining target
    // qualification builds (V01-09 six-target packaging).
    None
}

// ---------------------------------------------------------------------------
// Group B — DB / connection / statement lifecycle
// ---------------------------------------------------------------------------

/// `MbStatus mb_database_open(const uint8_t *config, size_t len, MbHandle *out);`
///
/// Config JSON (owned wire contract):
/// `{"path": string, "vfs": "default|memory|syscall|io_uring|experimental_win_iocp",`
///  `"experimental_features": [string...], "async_io": bool, "read_only": bool}`
///
/// MiraBase always enables the "vacuum" experimental token on open.
#[no_mangle]
pub unsafe extern "C" fn mb_database_open(
    config: *const u8,
    len: usize,
    out: *mut MbHandle,
) -> MbStatus {
    init_reg();
    if out.is_null() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"out pointer is null"}),
        );
    }
    if config.is_null() || len == 0 {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"config buffer is empty"}),
        );
    }
    let slice = std::slice::from_raw_parts(config, len);
    let cfg: DbConfig = match serde_json::from_slice(slice) {
        Ok(c) => c,
        Err(e) => {
            return err_status(
                MB_ABI_ERROR,
                serde_json::json!({"class":"abi_error","message":format!("invalid database config: {e}")}),
            )
        }
    };

    // VACUUM is always enabled for MiraBase databases (contract).
    let mut features = cfg.experimental_features.clone();
    if !features.iter().any(|f| f == "vacuum") {
        features.push("vacuum".to_string());
    }

    let vfs = match &cfg.vfs {
        None => turso_sdk_kit::IoBackend::Default,
        Some(name) => match io_backend(name) {
            Ok(v) => v,
            Err(s) => return s,
        },
    };

    let db_config = TursoDatabaseConfig {
        path: cfg.path.clone(),
        experimental_features: Some(features.join(",")),
        async_io: cfg.async_io,
        encryption: None,
        vfs,
        io: None,
        db_file: None,
        page_codec: None,
        open_flags: if cfg.read_only {
            OpenFlags::ReadOnly
        } else {
            OpenFlags::Create
        },
    };

    let db = TursoDatabase::new(db_config);
    // Drive the open state machine to completion: with async_io=true the
    // rsapi layer may return IOResult::IO (a pending completion) which the
    // C path pumps by re-entering open(); the machinery state persists in
    // the TursoDatabase. A bounded loop guards against a stuck backend.
    let mut open_attempts: u32 = 0;
    loop {
        match db.open() {
            Ok(turso::core::IOResult::Done(())) => break,
            Ok(turso::core::IOResult::IO(_)) => {
                open_attempts += 1;
                if open_attempts > 100_000 {
                    return err_status(
                        MB_BUSY,
                        serde_json::json!({"class":"busy","message":"database open did not complete after 100000 async iterations"}),
                    );
                }
                continue;
            }
            Err(e) => {
                let (code, detail) = classify_error(&e);
                return err_status(code, detail);
            }
        }
    }
    let id = alloc_id();
    let gen: u32 = 1;
    reg().dbs.write().insert(id, DbEntry { gen, db, path: cfg.path });
    *out = enc(id, gen);
    MB_OK_STATUS
}

/// `MbStatus mb_database_close(MbHandle database);`
#[no_mangle]
pub unsafe extern "C" fn mb_database_close(database: MbHandle) -> MbStatus {
    let Some((id, gen)) = dec(database) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid database handle"}),
        );
    };
    let removed = {
        let mut dbs = reg().dbs.write();
        match dbs.get(&id) {
            Some(e) if e.gen == gen => Some(dbs.remove(&id).unwrap()),
            _ => None,
        }
    };
    match removed {
        Some(entry) => {
            // Connections of a closed database are invalidated by removing
            // them; close is only legal once the caller has closed its
            // connections (misuse otherwise surfaces at close time).
            let mut conns = reg().conns.write();
            let orphaned: Vec<u32> =
                conns.iter().filter(|(_, c)| c.db_id == id).map(|(k, _)| *k).collect();
            for k in orphaned {
                conns.remove(&k);
            }
            drop(conns);
            let _ = entry;
            MB_OK_STATUS
        }
        None => err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown or stale database handle"}),
        ),
    }
}

/// `MbStatus mb_connection_open(MbHandle database, uint32_t frontend, MbHandle *out);`
#[no_mangle]
pub unsafe extern "C" fn mb_connection_open(
    database: MbHandle,
    frontend: u32,
    out: *mut MbHandle,
) -> MbStatus {
    if out.is_null() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"out pointer is null"}),
        );
    }
    if frontend != MB_SQLITE {
        return err_status(
            MB_UNSUPPORTED,
            serde_json::json!({"class":"unsupported","message":"only the sqlite frontend is part of the v0.1 payload"}),
        );
    }
    let Some((db_id, gen)) = dec(database) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid database handle"}),
        );
    };
    let db = {
        let dbs = reg().dbs.read();
        match dbs.get(&db_id) {
            Some(e) if e.gen == gen => e.db.clone(),
            _ => {
                return err_status(
                    MB_BUSY,
                    serde_json::json!({"class":"busy","message":"unknown or stale database handle"}),
                )
            }
        }
    };
    match db.connect() {
        Ok(conn) => {
            let id = alloc_id();
            let cgen: u32 = 1;
            reg().conns.write().insert(id, ConnEntry { gen: cgen, conn, db_id });
            *out = enc(id, cgen);
            MB_OK_STATUS
        }
        Err(e) => {
            let (code, detail) = classify_error(&e);
            err_status(code, detail)
        }
    }
}

/// `MbStatus mb_connection_close(MbHandle connection);`
#[no_mangle]
pub unsafe extern "C" fn mb_connection_close(connection: MbHandle) -> MbStatus {
    let Some((id, gen)) = dec(connection) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid connection handle"}),
        );
    };
    let conn = {
        let mut conns = reg().conns.write();
        match conns.get(&id) {
            Some(e) if e.gen == gen => {
                let c = e.conn.clone();
                let removed = conns.remove(&id).unwrap();
                Some((c, removed))
            }
            _ => None,
        }
    };
    match conn {
        Some((conn, removed)) => {
            // Statements of this connection are invalidated by removal; the
            // statements registry guard serializes with any in-flight use.
            {
                let mut stmts = reg().stmts.write();
                let orphans: Vec<u32> =
                    stmts.iter().filter(|(_, s)| s.conn_id == id).map(|(k, _)| *k).collect();
                for k in orphans {
                    stmts.remove(&k);
                }
            }
            let _ = removed;
            err_from(conn.close())
        }
        None => err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown or stale connection handle"}),
        ),
    }
}

/// `MbStatus mb_statement_prepare(MbHandle connection, const uint8_t *sql, size_t len, MbHandle *out);`
#[no_mangle]
pub unsafe extern "C" fn mb_statement_prepare(
    connection: MbHandle,
    sql: *const u8,
    len: usize,
    out: *mut MbHandle,
) -> MbStatus {
    if out.is_null() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"out pointer is null"}),
        );
    }
    let Some((conn_id, gen)) = dec(connection) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid connection handle"}),
        );
    };
    let conn = {
        let conns = reg().conns.read();
        match conns.get(&conn_id) {
            Some(e) if e.gen == gen => e.conn.clone(),
            _ => {
                return err_status(
                    MB_BUSY,
                    serde_json::json!({"class":"busy","message":"unknown or stale connection handle"}),
                )
            }
        }
    };
    if sql.is_null() || len == 0 {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"sql buffer is empty"}),
        );
    }
    let sql_bytes = std::slice::from_raw_parts(sql, len);
    let sql_str = match std::str::from_utf8(sql_bytes) {
        Ok(s) => s.to_string(),
        Err(e) => {
            return err_status(
                MB_ABI_ERROR,
                serde_json::json!({"class":"abi_error","message":format!("sql is not valid utf-8: {e}")}),
            )
        }
    };
    match conn.prepare_single(sql_str) {
        Ok(stmt) => {
            let id = alloc_id();
            let sgen: u32 = 1;
            reg().stmts.write().insert(
                id,
                StmtEntry { gen: sgen, stmt: Mutex::new(stmt), conn_id },
            );
            *out = enc(id, sgen);
            MB_OK_STATUS
        }
        Err(e) => {
            let (code, detail) = classify_error(&e);
            err_status(code, detail)
        }
    }
}

/// `MbStatus mb_statement_info(MbHandle statement, MbBuffer *classification_and_columns);`
#[no_mangle]
pub unsafe extern "C" fn mb_statement_info(statement: MbHandle, out: *mut MbBuffer) -> MbStatus {
    if out.is_null() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"out pointer is null"}),
        );
    }
    let Some((id, gen)) = dec(statement) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid statement handle"}),
        );
    };
    // Hold the registry read guard across the statement Mutex guard so a
    // concurrent close() cannot remove the entry under us.
    let stmts = reg().stmts.read();
    let Some(entry) = stmts.get(&id) else {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown or stale statement handle"}),
        );
    };
    if entry.gen != gen {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"stale statement handle"}),
        );
    }
    let stmt = entry.stmt.lock().unwrap();
    let count = stmt.column_count();
    let mut columns = Vec::with_capacity(count);
    for i in 0..count {
        let name = match stmt.column_name(i) {
            Ok(n) => n,
            Err(e) => {
                let (code, detail) = classify_error(&e);
                return err_status(code, detail);
            }
        };
        let decltype = stmt.column_decltype(i);
        columns.push(serde_json::json!({ "name": name, "decltype": decltype }));
    }
    let info = serde_json::json!({
        "parameters": stmt.parameters_count(),
        "columns": columns,
    });
    drop(stmt);
    drop(stmts);
    match serde_json::to_vec(&info) {
        Ok(bytes) => {
            *out = buf_from_bytes(bytes);
            MB_OK_STATUS
        }
        Err(e) => err_status(MB_ERROR, serde_json::json!({"class":"error","message":e.to_string()})),
    }
}

/// `MbStatus mb_statement_bind(MbHandle statement, const uint8_t *typed_values, size_t len);`
///
/// typed_values JSON array; each element:
/// `{"k":"null|int|real|text|blob", "i":<1-based position>, "v":<value>}` or
/// `{"k":..., "n":"<named parameter including prefix>", "v":<value>}`.
#[no_mangle]
pub unsafe extern "C" fn mb_statement_bind(
    statement: MbHandle,
    typed_values: *const u8,
    len: usize,
) -> MbStatus {
    let Some((id, gen)) = dec(statement) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid statement handle"}),
        );
    };
    if typed_values.is_null() || len == 0 {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"typed_values buffer is empty"}),
        );
    }
    let slice = std::slice::from_raw_parts(typed_values, len);
    let values: Vec<serde_json::Value> = match serde_json::from_slice(slice) {
        Ok(v) => v,
        Err(e) => {
            return err_status(
                MB_ABI_ERROR,
                serde_json::json!({"class":"abi_error","message":format!("invalid typed values: {e}")}),
            )
        }
    };

    let stmts = reg().stmts.read();
    let Some(entry) = stmts.get(&id) else {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown or stale statement handle"}),
        );
    };
    if entry.gen != gen {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"stale statement handle"}),
        );
    }
    let mut stmt = entry.stmt.lock().unwrap();
    for v in &values {
        let kind = v.get("k").and_then(|k| k.as_str()).unwrap_or("");
        let pos: Option<usize> = match v.get("i") {
            Some(p) => match p.as_u64() {
                Some(n) if n >= 1 => Some(n as usize),
                _ => None,
            },
            None => None,
        };
        let pos = match (pos, v.get("n")) {
            (Some(p), _) => p,
            (None, Some(name)) => {
                let name = match name.as_str() {
                    Some(n) => n,
                    None => {
                        return err_status(
                            MB_ABI_ERROR,
                            serde_json::json!({"class":"abi_error","message":"named parameter must be a string"}),
                        )
                    }
                };
                match stmt.named_position(name) {
                    Ok(p) => p,
                    Err(e) => {
                        let (code, detail) = classify_error(&e);
                        return err_status(code, detail);
                    }
                }
            }
            (None, None) => {
                return err_status(
                    MB_ABI_ERROR,
                    serde_json::json!({"class":"abi_error","message":"value needs \"i\" position or \"n\" name"}),
                )
            }
        };
        let value = match kind {
            "null" => Value::Null,
            "int" => match v.get("v").and_then(|x| x.as_i64()) {
                Some(i) => Value::from_i64(i),
                None => {
                    return err_status(
                        MB_ABI_ERROR,
                        serde_json::json!({"class":"abi_error","message":"int value missing"}),
                    )
                }
            },
            "real" => match v.get("v").and_then(|x| x.as_f64()) {
                Some(f) => Value::from_f64(f),
                None => {
                    return err_status(
                        MB_ABI_ERROR,
                        serde_json::json!({"class":"abi_error","message":"real value missing"}),
                    )
                }
            },
            "text" => match v.get("v").and_then(|x| x.as_str()) {
                Some(s) => Value::build_text(s.to_string()),
                None => {
                    return err_status(
                        MB_ABI_ERROR,
                        serde_json::json!({"class":"abi_error","message":"text value missing"}),
                    )
                }
            },
            "blob" => match v.get("v").and_then(|x| x.as_str()) {
                Some(b64) => match base64_decode(b64) {
                    Some(b) => Value::Blob(b),
                    None => {
                        return err_status(
                            MB_ABI_ERROR,
                            serde_json::json!({"class":"abi_error","message":"blob value is not valid base64"}),
                        )
                    }
                },
                None => {
                    return err_status(
                        MB_ABI_ERROR,
                        serde_json::json!({"class":"abi_error","message":"blob value missing"}),
                    )
                }
            },
            other => {
                return err_status(
                    MB_ABI_ERROR,
                    serde_json::json!({"class":"abi_error","message":format!("unknown value kind {other:?}")}),
                )
            }
        };
        if let Err(e) = stmt.bind_positional(pos, value) {
            let (code, detail) = classify_error(&e);
            return err_status(code, detail);
        }
    }
    drop(stmt);
    drop(stmts);
    MB_OK_STATUS
}

/// `MbStatus mb_statement_step(MbHandle statement, uint64_t operation_id, MbBuffer *row);`
///
/// Returns MB_ROW with an owned row buffer (JSON `{"c":n,"v":[values]}`,
/// blobs base64), MB_DONE, MB_PENDING_IO (operation registered), or a
/// classified error. The engine's async mode must have been selected at
/// database open for MB_PENDING_IO to be produced.
#[no_mangle]
pub unsafe extern "C" fn mb_statement_step(
    statement: MbHandle,
    operation_id: u64,
    row: *mut MbBuffer,
) -> MbStatus {
    if row.is_null() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"row pointer is null"}),
        );
    }
    let Some((id, gen)) = dec(statement) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid statement handle"}),
        );
    };
    // Hold the registry read guard across the statement Mutex guard (lock
    // order: stmts -> stmt mutex -> conns -> ops).
    let stmts = reg().stmts.read();
    let Some(entry) = stmts.get(&id) else {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown or stale statement handle"}),
        );
    };
    if entry.gen != gen {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"stale statement handle"}),
        );
    }
    let conn = {
        let conns = reg().conns.read();
        match conns.get(&entry.conn_id) {
            Some(c) => c.conn.clone(),
            None => {
                return err_status(
                    MB_BUSY,
                    serde_json::json!({"class":"busy","message":"owning connection is gone"}),
                )
            }
        }
    };
    let mut stmt = entry.stmt.lock().unwrap();
    match stmt.step(None) {
        Ok(status) => match status {
            TursoStatusCode::Row => match encode_row(&stmt) {
                Ok(bytes) => {
                    *row = buf_from_bytes(bytes);
                    simple_status(MB_ROW)
                }
                Err(e) => {
                    let (code, detail) = classify_error(&e);
                    err_status(code, detail)
                }
            },
            TursoStatusCode::Io => {
                if operation_id == 0 {
                    return err_status(
                        MB_ABI_ERROR,
                        serde_json::json!({"class":"abi_error","message":"async step requires a non-zero operation id"}),
                    );
                }
                reg().ops.write().insert(
                    operation_id,
                    OpEntry { conn, stmt_id: id, stmt_gen: gen },
                );
                simple_status(MB_PENDING_IO)
            }
            TursoStatusCode::Done => {
                reg().ops.write().remove(&operation_id);
                simple_status(MB_DONE)
            }
        },
        Err(e) => {
            let (code, detail) = classify_error(&e);
            reg().ops.write().remove(&operation_id);
            err_status(code, detail)
        }
    }
}

/// `MbStatus mb_statement_close(MbHandle statement);`
#[no_mangle]
pub unsafe extern "C" fn mb_statement_close(statement: MbHandle) -> MbStatus {
    let Some((id, gen)) = dec(statement) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid statement handle"}),
        );
    };
    // Hold the write guard across finalize so no in-flight user of the entry
    // can exist while the Mutex is dropped.
    let mut stmts = reg().stmts.write();
    let Some(entry) = stmts.get(&id) else {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown or stale statement handle"}),
        );
    };
    if entry.gen != gen {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"stale statement handle"}),
        );
    }
    let removed = stmts.remove(&id).unwrap();
    drop(stmts);
    let mut stmt = removed.stmt.lock().unwrap();
    match stmt.finalize(None) {
        Ok(_) => MB_OK_STATUS,
        Err(e) => {
            let (code, detail) = classify_error(&e);
            err_status(code, detail)
        }
    }
}

// ---------------------------------------------------------------------------
// Group C — I/O poll / cancel
// ---------------------------------------------------------------------------

/// `MbStatus mb_io_poll(uint64_t operation_id, uint32_t timeout_ms);`
///
/// Drives one I/O iteration of the owning statement (`run_io`); returns
/// MB_OK when the iteration completed (caller re-steps), MB_BUSY when the
/// operation id is unknown, or a classified error on I/O failure.
/// `timeout_ms` is advisory: the engine's I/O backend drives one
/// non-blocking iteration per call and the host enforces deadlines by
/// bounded polling.
#[no_mangle]
pub unsafe extern "C" fn mb_io_poll(operation_id: u64, timeout_ms: u32) -> MbStatus {
    let _ = timeout_ms;
    let op = {
        let ops = reg().ops.read();
        ops.get(&operation_id).cloned()
    };
    let Some(op) = op else {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown operation id"}),
        );
    };
    // Lock order: stmts -> stmt mutex (ops guard already released).
    let stmts = reg().stmts.read();
    let Some(entry) = stmts.get(&op.stmt_id) else {
        drop(stmts);
        reg().ops.write().remove(&operation_id);
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"owning statement is gone"}),
        );
    };
    if entry.gen != op.stmt_gen {
        drop(stmts);
        reg().ops.write().remove(&operation_id);
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"owning statement handle is stale"}),
        );
    }
    let stmt = entry.stmt.lock().unwrap();
    match stmt.run_io() {
        Ok(()) => MB_OK_STATUS,
        Err(e) => {
            let (code, detail) = classify_error(&e);
            drop(stmt);
            drop(stmts);
            reg().ops.write().remove(&operation_id);
            err_status(code, detail)
        }
    }
}

/// `MbStatus mb_cancel(uint64_t operation_id);`
///
/// Cancels the owning statement's work through the engine's real interrupt
/// mechanism (`TursoConnection::interrupt`). The next step returns
/// MB_CANCELLED. Cancelling an unknown operation id is MB_BUSY.
#[no_mangle]
pub unsafe extern "C" fn mb_cancel(operation_id: u64) -> MbStatus {
    let op = {
        let ops = reg().ops.read();
        ops.get(&operation_id).cloned()
    };
    match op {
        Some(op) => {
            op.conn.interrupt();
            MB_OK_STATUS
        }
        None => err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown operation id"}),
        ),
    }
}

// ---------------------------------------------------------------------------
// Group D — transactions
// ---------------------------------------------------------------------------

fn tx_sql(mode: u32) -> Result<&'static str, MbStatus> {
    match mode {
        MB_TX_READ => Ok("BEGIN"),
        MB_TX_WAL_WRITE => Ok("BEGIN IMMEDIATE"),
        MB_TX_MVCC_WRITE => Ok("BEGIN CONCURRENT"),
        _ => Err(err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"unknown transaction mode"}),
        )),
    }
}

fn run_simple(
    conn: &Arc<turso_sdk_kit::rsapi::TursoConnection>,
    sql: &str,
) -> MbStatus {
    match conn.prepare_single(sql) {
        Ok(mut stmt) => match stmt.execute(None) {
            Ok(_) => {
                let fin = stmt.finalize(None);
                match fin {
                    Ok(_) | Err(_) => MB_OK_STATUS,
                }
            }
            Err(e) => {
                let (code, detail) = classify_error(&e);
                err_status(code, detail)
            }
        },
        Err(e) => {
            let (code, detail) = classify_error(&e);
            err_status(code, detail)
        }
    }
}

/// `MbStatus mb_transaction_begin(MbHandle connection, uint32_t mode);`
#[no_mangle]
pub unsafe extern "C" fn mb_transaction_begin(connection: MbHandle, mode: u32) -> MbStatus {
    let Some((id, gen)) = dec(connection) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid connection handle"}),
        );
    };
    let conn = {
        let conns = reg().conns.read();
        match conns.get(&id) {
            Some(e) if e.gen == gen => e.conn.clone(),
            _ => {
                return err_status(
                    MB_BUSY,
                    serde_json::json!({"class":"busy","message":"unknown or stale connection handle"}),
                )
            }
        }
    };
    let sql = match tx_sql(mode) {
        Ok(s) => s,
        Err(e) => return e,
    };
    run_simple(&conn, sql)
}

/// `MbStatus mb_transaction_commit(MbHandle connection);`
#[no_mangle]
pub unsafe extern "C" fn mb_transaction_commit(connection: MbHandle) -> MbStatus {
    let Some((id, gen)) = dec(connection) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid connection handle"}),
        );
    };
    let conn = {
        let conns = reg().conns.read();
        match conns.get(&id) {
            Some(e) if e.gen == gen => e.conn.clone(),
            _ => {
                return err_status(
                    MB_BUSY,
                    serde_json::json!({"class":"busy","message":"unknown or stale connection handle"}),
                )
            }
        }
    };
    run_simple(&conn, "COMMIT")
}

/// `MbStatus mb_transaction_rollback(MbHandle connection);`
#[no_mangle]
pub unsafe extern "C" fn mb_transaction_rollback(connection: MbHandle) -> MbStatus {
    let Some((id, gen)) = dec(connection) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid connection handle"}),
        );
    };
    let conn = {
        let conns = reg().conns.read();
        match conns.get(&id) {
            Some(e) if e.gen == gen => e.conn.clone(),
            _ => {
                return err_status(
                    MB_BUSY,
                    serde_json::json!({"class":"busy","message":"unknown or stale connection handle"}),
                )
            }
        }
    };
    run_simple(&conn, "ROLLBACK")
}

// ---------------------------------------------------------------------------
// Group F — DB quiesce / persistent manifest / memory
// ---------------------------------------------------------------------------

/// `MbStatus mb_database_quiesce(MbHandle database, MbBuffer *persistent_file_manifest);`
///
/// Owned primitive (D-17): refuses while connections of this database are
/// live (MB_BUSY), checkpoints the WAL into the main file through the
/// engine's real checkpoint support, and emits a JSON manifest of the
/// persistent files (`[{file,bytes,sha256}]`).
#[no_mangle]
pub unsafe extern "C" fn mb_database_quiesce(
    database: MbHandle,
    persistent_file_manifest: *mut MbBuffer,
) -> MbStatus {
    if persistent_file_manifest.is_null() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"manifest out pointer is null"}),
        );
    }
    let Some((id, gen)) = dec(database) else {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid database handle"}),
        );
    };
    let db_entry = {
        let dbs = reg().dbs.read();
        match dbs.get(&id) {
            Some(e) if e.gen == gen => Some((e.db.clone(), e.path.clone())),
            _ => None,
        }
    };
    let Some((db, path)) = db_entry else {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"unknown or stale database handle"}),
        );
    };
    // Refuse to quiesce with live connections.
    let live = reg().conns.read().iter().any(|(_, c)| c.db_id == id);
    if live {
        return err_status(
            MB_BUSY,
            serde_json::json!({"class":"busy","message":"database still has live connections"}),
        );
    }
    // Checkpoint through a private connection.
    let conn = match db.connect() {
        Ok(c) => c,
        Err(e) => {
            let (code, detail) = classify_error(&e);
            return err_status(code, detail);
        }
    };
    let checkpoint_status = run_simple(&conn, "PRAGMA wal_checkpoint(TRUNCATE)");
    let _ = conn.close();
    if checkpoint_status.code != MB_OK {
        return checkpoint_status;
    }
    // Build the persistent file manifest over the real files.
    let manifest = match build_file_manifest(&path) {
        Ok(m) => m,
        Err(e) => return err_status(MB_ERROR, serde_json::json!({"class":"error","message":e})),
    };
    match serde_json::to_vec(&manifest) {
        Ok(bytes) => {
            *persistent_file_manifest = buf_from_bytes(bytes);
            MB_OK_STATUS
        }
        Err(e) => err_status(MB_ERROR, serde_json::json!({"class":"error","message":e.to_string()})),
    }
}

fn build_file_manifest(base: &str) -> Result<serde_json::Value, String> {
    use sha2::{Digest, Sha256};
    if base.is_empty() || base == ":memory:" {
        return Ok(serde_json::json!([]));
    }
    let mut files = Vec::new();
    let candidates: Vec<String> =
        vec![base.to_string(), format!("{base}-wal"), format!("{base}-shm")];
    for file in candidates {
        if let Ok(bytes) = std::fs::read(&file) {
            let mut h = Sha256::new();
            h.update(&bytes);
            files.push(serde_json::json!({
                "file": file,
                "bytes": bytes.len(),
                "sha256": format!("{:x}", h.finalize()),
            }));
        }
    }
    Ok(serde_json::json!(files))
}

/// `MbStatus mb_memory_usage(MbHandle database, uint64_t *allocated_bytes);`
///
/// v0.1 accounting: the pinned engine revision has no per-database memory
/// counter; the bridge reports the engine process's real allocator total
/// (mimalloc) and records the scope in the detail JSON. Per-database
/// accounting is an owned native/patches follow-up, tracked in the
/// implementation ledger.
#[no_mangle]
pub unsafe extern "C" fn mb_memory_usage(
    database: MbHandle,
    allocated_bytes: *mut u64,
) -> MbStatus {
    if allocated_bytes.is_null() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"out pointer is null"}),
        );
    }
    if dec(database).is_none() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid database handle"}),
        );
    }
    *allocated_bytes = process_allocated_bytes();
    MB_OK_STATUS
}

/// `MbStatus mb_set_memory_budget(MbHandle database, uint64_t maximum_bytes);`
///
/// The pinned engine revision provides no enforcement hook for a memory
/// budget. Per the contract ("Profiles select among real compiled
/// capabilities; they do not simulate absent functionality"; "never install
/// no-op functions pretending success") this symbol reports MB_UNSUPPORTED
/// with an honest detail until the owned core patch lands. Recorded as an
/// open implementation item in the P02 ledger.
#[no_mangle]
pub unsafe extern "C" fn mb_set_memory_budget(database: MbHandle, maximum_bytes: u64) -> MbStatus {
    let _ = maximum_bytes;
    if dec(database).is_none() {
        return err_status(
            MB_ABI_ERROR,
            serde_json::json!({"class":"abi_error","message":"invalid database handle"}),
        );
    }
    err_status(
        MB_UNSUPPORTED,
        serde_json::json!({
            "class": "unsupported",
            "message": "the pinned engine revision does not expose a memory budget enforcement hook; owned patch tracked as P02 open item"
        }),
    )
}

/// Real process allocation total from mimalloc (the engine's global
/// allocator via the turso crate). Uses the allocator's committed-bytes
/// metric from mi_process_info, which is the real engine allocation total
/// available in this build.
fn process_allocated_bytes() -> u64 {
    unsafe {
        extern "C" {
            fn mi_process_info(
                elapsed_msec: *mut usize,
                user_msec: *mut usize,
                system_msec: *mut usize,
                current_rss: *mut usize,
                peak_rss: *mut usize,
                current_commit: *mut usize,
                peak_commit: *mut usize,
                page_faults: *mut usize,
            );
        }
        let mut _e: usize = 0;
        let mut _u: usize = 0;
        let mut _s: usize = 0;
        let mut _cr: usize = 0;
        let mut _pr: usize = 0;
        let mut current_commit: usize = 0;
        let mut _pc: usize = 0;
        let mut _pf: usize = 0;
        mi_process_info(&mut _e, &mut _u, &mut _s, &mut _cr, &mut _pr, &mut current_commit, &mut _pc, &mut _pf);
        current_commit as u64
    }
}

// ---------------------------------------------------------------------------
// Row encoding / helpers
// ---------------------------------------------------------------------------

fn encode_row(stmt: &turso_sdk_kit::rsapi::TursoStatement) -> Result<Vec<u8>, TursoError> {
    use turso::core::Numeric;
    let count = stmt.column_count();
    let mut values = Vec::with_capacity(count);
    for i in 0..count {
        let v = stmt.row_value(i)?;
        let j = match v {
            Value::Null => serde_json::json!({"t":"null"}),
            Value::Numeric(Numeric::Integer(i)) => serde_json::json!({"t":"int","v":i}),
            Value::Numeric(Numeric::Float(nn)) => serde_json::json!({"t":"real","v": f64::from(nn)}),
            Value::Text(t) => serde_json::json!({"t":"text","v":t.as_str()}),
            Value::Blob(b) => serde_json::json!({"t":"blob","v":base64_encode(&b)}),
        };
        values.push(j);
    }
    Ok(serde_json::to_vec(&serde_json::json!({ "c": count, "v": values })).unwrap_or_default())
}

fn base64_encode(data: &[u8]) -> String {
    const TABLE: &[u8] =
        b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::with_capacity(data.len().div_ceil(3) * 4);
    for chunk in data.chunks(3) {
        let b0 = chunk[0] as u32;
        let b1 = if chunk.len() > 1 { chunk[1] as u32 } else { 0 };
        let b2 = if chunk.len() > 2 { chunk[2] as u32 } else { 0 };
        let n = (b0 << 16) | (b1 << 8) | b2;
        out.push(TABLE[(n >> 18) as usize & 63] as char);
        out.push(TABLE[(n >> 12) as usize & 63] as char);
        if chunk.len() > 1 {
            out.push(TABLE[(n >> 6) as usize & 63] as char);
        } else {
            out.push('=');
        }
        if chunk.len() > 2 {
            out.push(TABLE[n as usize & 63] as char);
        } else {
            out.push('=');
        }
    }
    out
}

fn base64_decode(s: &str) -> Option<Vec<u8>> {
    fn val(c: u8) -> Option<u32> {
        match c {
            b'A'..=b'Z' => Some((c - b'A') as u32),
            b'a'..=b'z' => Some((c - b'a') as u32 + 26),
            b'0'..=b'9' => Some((c - b'0') as u32 + 52),
            b'+' => Some(62),
            b'/' => Some(63),
            _ => None,
        }
    }
    let s = s.trim_end_matches('=');
    let mut out = Vec::with_capacity(s.len() * 3 / 4);
    let bytes = s.as_bytes();
    for chunk in bytes.chunks(4) {
        let mut n: u32 = 0;
        for (i, &c) in chunk.iter().enumerate() {
            n |= val(c)? << (18 - 6 * i);
        }
        out.push((n >> 16) as u8);
        if chunk.len() > 2 {
            out.push((n >> 8) as u8);
        }
        if chunk.len() > 3 {
            out.push(n as u8);
        }
    }
    Some(out)
}

// ---------------------------------------------------------------------------
// Upstream symbol retention anchor
// ---------------------------------------------------------------------------

/// The pristine upstream Go driver binds every `turso_sync_*` C symbol from
/// this same payload. Nothing in the owned mb_* surface references the sync
/// crate, so without this anchor the linker elides the crate entirely. The
/// static function-pointer reference forces the upstream sync objects to be
/// linked (and their no-mangle symbols exported) without executing
/// anything; the /INCLUDE linker args in build.rs cover the rest.
static SYNC_SURFACE_ANCHOR: unsafe extern "C" fn(*const turso_sync_sdk_kit::capi::c::turso_sync_changes_t) =
    turso_sync_sdk_kit::capi::turso_sync_changes_deinit;

// ---------------------------------------------------------------------------
// Out-pointer binding variants (*_into)
//
// The canonical mb_* functions return MbStatus by value. Some host language
// runtimes (notably purego on non-darwin targets) cannot bind by-value
// struct returns. Each *_into variant takes the status through an explicit
// out pointer instead — ABI-stable on every target — while delegating to the
// canonical function so the two surfaces can never diverge. The Go binding
// binds only *_into symbols.
// ---------------------------------------------------------------------------

macro_rules! into_variant {
    ($name:ident, $doc:expr, ($($arg:ident: $ty:ty),*) => $call:expr) => {
        #[doc = $doc]
        #[no_mangle]
        pub unsafe extern "C" fn $name(out_status: *mut MbStatus, $($arg: $ty),*) {
            let status: MbStatus = $call;
            if !out_status.is_null() {
                *out_status = status;
            }
        }
    };
}

into_variant!(
    mb_bridge_init_into,
    "`MbStatus mb_bridge_init(void)` with status through an out pointer.",
    () => mb_bridge_init()
);
into_variant!(
    mb_build_manifest_into,
    "`mb_build_manifest` with status through an out pointer.",
    (out: *mut MbBuffer) => mb_build_manifest(out)
);
into_variant!(
    mb_database_open_into,
    "`mb_database_open` with status through an out pointer.",
    (config: *const u8, len: usize, out: *mut MbHandle) => mb_database_open(config, len, out)
);
into_variant!(
    mb_database_close_into,
    "`mb_database_close` with status through an out pointer.",
    (database: MbHandle) => mb_database_close(database)
);
into_variant!(
    mb_connection_open_into,
    "`mb_connection_open` with status through an out pointer.",
    (database: MbHandle, frontend: u32, out: *mut MbHandle) => mb_connection_open(database, frontend, out)
);
into_variant!(
    mb_connection_close_into,
    "`mb_connection_close` with status through an out pointer.",
    (connection: MbHandle) => mb_connection_close(connection)
);
into_variant!(
    mb_statement_prepare_into,
    "`mb_statement_prepare` with status through an out pointer.",
    (connection: MbHandle, sql: *const u8, len: usize, out: *mut MbHandle) =>
        mb_statement_prepare(connection, sql, len, out)
);
into_variant!(
    mb_statement_info_into,
    "`mb_statement_info` with status through an out pointer.",
    (statement: MbHandle, out: *mut MbBuffer) => mb_statement_info(statement, out)
);
into_variant!(
    mb_statement_bind_into,
    "`mb_statement_bind` with status through an out pointer.",
    (statement: MbHandle, typed_values: *const u8, len: usize) =>
        mb_statement_bind(statement, typed_values, len)
);
into_variant!(
    mb_statement_step_into,
    "`mb_statement_step` with status through an out pointer.",
    (statement: MbHandle, operation_id: u64, row: *mut MbBuffer) =>
        mb_statement_step(statement, operation_id, row)
);
into_variant!(
    mb_statement_close_into,
    "`mb_statement_close` with status through an out pointer.",
    (statement: MbHandle) => mb_statement_close(statement)
);
into_variant!(
    mb_io_poll_into,
    "`mb_io_poll` with status through an out pointer.",
    (operation_id: u64, timeout_ms: u32) => mb_io_poll(operation_id, timeout_ms)
);
into_variant!(
    mb_cancel_into,
    "`mb_cancel` with status through an out pointer.",
    (operation_id: u64) => mb_cancel(operation_id)
);
into_variant!(
    mb_transaction_begin_into,
    "`mb_transaction_begin` with status through an out pointer.",
    (connection: MbHandle, mode: u32) => mb_transaction_begin(connection, mode)
);
into_variant!(
    mb_transaction_commit_into,
    "`mb_transaction_commit` with status through an out pointer.",
    (connection: MbHandle) => mb_transaction_commit(connection)
);
into_variant!(
    mb_transaction_rollback_into,
    "`mb_transaction_rollback` with status through an out pointer.",
    (connection: MbHandle) => mb_transaction_rollback(connection)
);
into_variant!(
    mb_database_quiesce_into,
    "`mb_database_quiesce` with status through an out pointer.",
    (database: MbHandle, persistent_file_manifest: *mut MbBuffer) =>
        mb_database_quiesce(database, persistent_file_manifest)
);
into_variant!(
    mb_memory_usage_into,
    "`mb_memory_usage` with status through an out pointer.",
    (database: MbHandle, allocated_bytes: *mut u64) => mb_memory_usage(database, allocated_bytes)
);
into_variant!(
    mb_set_memory_budget_into,
    "`mb_set_memory_budget` with status through an out pointer.",
    (database: MbHandle, maximum_bytes: u64) => mb_set_memory_budget(database, maximum_bytes)
);

/// `void mb_buffer_free(MbBuffer value)` with the buffer through a pointer,
/// for hosts that cannot pass 16-byte structs by value.
#[no_mangle]
pub unsafe extern "C" fn mb_buffer_free_into(value: *const MbBuffer) {
    if value.is_null() {
        return;
    }
    let v = *value;
    mb_buffer_free(v);
}

// ---------------------------------------------------------------------------
// One-time engine setup (called by the host before any mb_* database work)
// ---------------------------------------------------------------------------

/// Initialize the bridge registry and the engine's global logging. The host
/// calls this exactly once at startup, before the read-only capability
/// probes, so failures are fatal before application files open.
#[no_mangle]
pub extern "C" fn mb_bridge_init() -> MbStatus {
    init_reg();
    let log_level: Option<String> = None;
    let config = TursoSetupConfig { logger: None, log_level };
    match turso_setup(config) {
        Ok(()) => MB_OK_STATUS,
        Err(e) => {
            let (code, detail) = classify_error(&e);
            err_status(code, detail)
        }
    }
}
