// MiraBase-owned build manifest emission for the native bridge payload.
// Records the exact build inputs of this payload so mb_build_manifest can
// report them truthfully at runtime.
use std::process::Command;
use std::path::Path;
use sha2::{Digest, Sha256};

fn verify_owned_patches() {
    let crate_dir = std::path::PathBuf::from(std::env::var("CARGO_MANIFEST_DIR").unwrap());
    let engine = crate_dir.join("../engine").canonicalize().expect("pinned engine checkout missing");
    let patches = crate_dir.join("../patches");
    let manifest_path = patches.join("manifest.json");
    let bytes = std::fs::read(&manifest_path).expect("owned patch manifest missing");
    let manifest: serde_json::Value = serde_json::from_slice(&bytes).expect("invalid patch manifest");
    let git = |args: &[&str]| {
        let out = Command::new("git").arg("-C").arg(&engine).args(args).output().expect("git unavailable");
        assert!(out.status.success(), "cannot inspect pinned engine inputs");
        out.stdout
    };
    let head = git(&["rev-parse", "HEAD"]);
    assert_eq!(String::from_utf8_lossy(&head).trim(), manifest["engineRevision"].as_str().unwrap(), "wrong engine commit");
    assert!(git(&["diff", "--cached", "--name-only"]).is_empty(), "unexpected staged engine changes");
    assert!(git(&["ls-files", "--others", "--exclude-standard"]).is_empty(), "unrecorded engine files");
    for patch in manifest["patches"].as_array().expect("patch list missing") {
        let name = patch["file"].as_str().unwrap();
        assert_eq!(Path::new(name).components().count(), 1, "patch path escaped manifest directory");
        let input = std::fs::read(patches.join(name)).expect("patch file missing");
        assert_eq!(format!("{:x}", Sha256::digest(&input)), patch["sha256"].as_str().unwrap(), "patch digest mismatch");
        println!("cargo:rerun-if-changed={}", patches.join(name).display());
    }
    let diff = git(&["diff", "--binary", "--no-ext-diff"]);
    assert_eq!(format!("{:x}", Sha256::digest(&diff)), manifest["appliedDiffSHA256"].as_str().unwrap(), "engine changes do not match the owned patch set");
    println!("cargo:rustc-env=MB_BUILD_PATCH_SET_DIGEST={:x}", Sha256::digest(&bytes));
    println!("cargo:rerun-if-changed={}", manifest_path.display());
}

fn main() {
    verify_owned_patches();
    let target = std::env::var("TARGET").unwrap_or_default();
    println!("cargo:rustc-env=MB_BUILD_TARGET={}", target);

    let features = std::env::var("CARGO_FEATURE_TURSO").is_ok();
    let _ = features;

    // Record the toolchain that compiled this payload.
    if let Ok(out) = Command::new("rustc").arg("--version").output() {
        let v = String::from_utf8_lossy(&out.stdout).trim().to_string();
        println!("cargo:rustc-env=MB_BUILD_RUSTC={}", v);
    }

    // The v0.1 payload feature set (see docs/mirabase/v0.1-build.md):
    // SQLite frontend, core functions, vector, FTS, WAL/MVCC, platform I/O.
    println!("cargo:rustc-env=MB_BUILD_FEATURES=sqlite-frontend,core,vector,fts,wal,mvcc,io_fallback");

    // Rerun if the pinned engine sources change (path dependencies).
    println!("cargo:rerun-if-changed=../engine/core");
    println!("cargo:rerun-if-changed=../engine/sdk-kit");
    println!("cargo:rerun-if-changed=../engine/sync/sdk-kit");

    // MSVC /OPT:REF drops unreferenced crate objects. The pristine upstream
    // Go driver binds every turso_*/turso_sync_* symbol below via purego, so
    // force each one to be retained (and thereby exported) in this payload.
    // ELF cdylibs (linux) export all no_mangle symbols from the linked crate
    // graph by default, so the /INCLUDE directives are MSVC-only.
    if std::env::var("CARGO_CFG_TARGET_ENV").as_deref() == Ok("msvc") {
        for symbol in upstream_c_abi_symbols() {
            println!("cargo:rustc-link-arg=/INCLUDE:{}", symbol);
        }
    }
}

/// The complete upstream C-ABI surface the pristine Go driver binds
/// (turso/bindings/go bindings_db.go + bindings_sync.go at the pinned
/// commit).
fn upstream_c_abi_symbols() -> Vec<&'static str> {
    let db = [
        "turso_setup",
        "turso_version",
        "turso_database_new",
        "turso_database_open",
        "turso_database_connect",
        "turso_database_deinit",
        "turso_connection_close",
        "turso_connection_deinit",
        "turso_connection_get_autocommit",
        "turso_connection_set_busy_timeout_ms",
        "turso_connection_last_insert_rowid",
        "turso_connection_prepare_single",
        "turso_connection_prepare_first",
        "turso_statement_execute",
        "turso_statement_step",
        "turso_statement_run_io",
        "turso_statement_reset",
        "turso_statement_finalize",
        "turso_statement_deinit",
        "turso_statement_n_change",
        "turso_statement_column_count",
        "turso_statement_column_name",
        "turso_statement_column_decltype",
        "turso_statement_column_declared_name",
        "turso_statement_column_kind",
        "turso_statement_column_base_type",
        "turso_statement_column_array_dimensions",
        "turso_statement_row_value_kind",
        "turso_statement_row_value_int",
        "turso_statement_row_value_double",
        "turso_statement_row_value_bytes_count",
        "turso_statement_row_value_bytes_ptr",
        "turso_statement_named_position",
        "turso_statement_parameters_count",
        "turso_statement_parameter_name",
        "turso_statement_bind_positional_null",
        "turso_statement_bind_positional_int",
        "turso_statement_bind_positional_double",
        "turso_statement_bind_positional_text",
        "turso_statement_bind_positional_blob",
        "turso_str_deinit",
        "turso_connection_enable_load_extension",
        "turso_connection_load_extension",
        "turso_connection_register_scalar_function",
        "turso_connection_register_aggregate_function",
        "turso_connection_unregister_function",
        "turso_connection_register_collation",
        "turso_connection_unregister_collation",
    ];
    let sync = [
        "turso_sync_database_new",
        "turso_sync_database_open",
        "turso_sync_database_create",
        "turso_sync_database_connect",
        "turso_sync_database_deinit",
        "turso_sync_database_stats",
        "turso_sync_database_push_changes",
        "turso_sync_database_apply_changes",
        "turso_sync_database_wait_changes",
        "turso_sync_database_checkpoint",
        "turso_sync_database_io_done",
        "turso_sync_database_io_poison",
        "turso_sync_database_io_push_buffer",
        "turso_sync_database_io_request_full_read",
        "turso_sync_database_io_request_full_write",
        "turso_sync_database_io_request_http",
        "turso_sync_database_io_request_http_header",
        "turso_sync_database_io_request_kind",
        "turso_sync_database_io_status",
        "turso_sync_database_io_step_callbacks",
        "turso_sync_database_io_take_item",
        "turso_sync_operation_resume",
        "turso_sync_operation_deinit",
        "turso_sync_operation_result_kind",
        "turso_sync_operation_result_extract_changes",
        "turso_sync_operation_result_extract_connection",
        "turso_sync_operation_result_extract_stats",
        "turso_sync_changes_deinit",
    ];
    let mut all = Vec::with_capacity(db.len() + sync.len());
    all.extend_from_slice(&db);
    all.extend_from_slice(&sync);
    all
}
