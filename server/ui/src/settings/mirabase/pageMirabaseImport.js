import { settingsSidebar } from "../settingsSidebar";

// pageMirabaseImport — the shared-layout PocketBase adoption screen
// (MB-UI slice): creates the import session, uploads the source
// directory snapshot (chunked, hash-verified), runs the analysis, drafts
// the shared plan, computes the preview, stages, validates and cuts over
// — the exact pbimport.* HTTP surface from V01-08.
export function pageMirabaseImport(route) {
    app.store.title = "PocketBase import";

    const data = store({
        isLoading: false,
        error: "",
        session: null,
        sessionPhase: "",
        sessionRevision: 1,
        analysis: null,
        planHash: "",
        preview: null,
        staged: null,
        validation: null,
        file: null,
        get sessionId() {
            return data.session?.id ?? "";
        },
        get phase() {
            return data.session?.phase ?? "";
        },
        get canUpload() {
            return data.file && (data.phase === "awaiting-source" || data.phase === "source-ready");
        },
        get canAnalyze() {
            return data.phase === "source-ready";
        },
        get canPlan() {
            return data.phase === "draft" && data.analysis;
        },
        get canPreview() {
            return data.phase === "draft" && data.planHash;
        },
        get canStage() {
            return data.phase === "previewed";
        },
        get canValidate() {
            return data.phase === "staged";
        },
        get canCutover() {
            return data.phase === "ready-for-cutover" && data.validation?.passed;
        },
    });

    async function api(path, method, body, rawBody) {
        const token = app.pb.authStore.token;
        const headers = {};
        if (!rawBody) {
            headers["Content-Type"] = "application/json";
        }
        if (token) {
            headers.Authorization = token;
        }
        const resp = await fetch("/api/mirabase/v1" + path, {
            method: method,
            headers: headers,
            body: rawBody ? rawBody : body === undefined ? undefined : JSON.stringify(body),
        });
        let payload;
        const text = await resp.text();
        try {
            payload = JSON.parse(text);
        } catch {
            payload = text;
        }
        if (!resp.ok) {
            const code = payload?.data?.mirabase?.code ?? "invalid_request";
            throw new Error(code + ": " + (payload.message ?? resp.status));
        }
        return payload;
    }

    async function refreshSession() {
        if (!data.sessionId) return;
        const sess = await api(`/pocketbase-imports/${data.sessionId}`, "GET");
        data.session = sess;
        data.sessionPhase = sess.phase;
        data.sessionRevision = sess.revision;
    }

    async function createSession() {
        if (!data.file) return;
        data.isLoading = true;
        data.error = "";
        try {
            const bytes = await data.file.arrayBuffer();
            const digest = await crypto.subtle.digest("SHA-256", bytes);
            const sha = [...new Uint8Array(digest)]
                .map((b) => b.toString(16).padStart(2, "0"))
                .join("");
            const session = await api("/pocketbase-imports", "POST", {
                sourceKind: "pocketbase-directory",
                sourceBytes: bytes.byteLength,
                sourceSha256: sha,
                targetMode: "new-application",
                expectedPrimaryGeneration: "1",
                sourceLabel: data.file.name,
            });
            data.session = session;
            data.sessionPhase = session.phase;
            data.sessionRevision = session.revision;
            data.analysis = null;
            data.planHash = "";
            data.preview = null;
            data.staged = null;
            data.validation = null;
            // Upload in 8 MiB chunks with hash verification.
            const chunkSize = 8 * 1024 * 1024;
            for (let offset = 0; offset < bytes.byteLength; offset += chunkSize) {
                const chunk = bytes.slice(offset, Math.min(offset + chunkSize, bytes.byteLength));
                const chunkHash = await crypto.subtle.digest("SHA-256", chunk);
                const chunkSha = [...new Uint8Array(chunkHash)]
                    .map((b) => b.toString(16).padStart(2, "0"))
                    .join("");
                await clientRequestUpload(session.id, offset, chunk, chunkSha, bytes.byteLength);
            }
            await refreshSession();
            app.toasts?.success?.("The source snapshot was uploaded and verified.");
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function clientRequestUpload(importId, offset, chunk, chunkSha, total) {
        const token = app.pb.authStore.token;
        const resp = await fetch(
            `/api/mirabase/v1/pocketbase-imports/${importId}/source`,
            {
                method: "PUT",
                headers: {
                    "Content-Type": "application/octet-stream",
                    "Content-Range": `bytes ${offset}-${offset + chunk.byteLength - 1}/${total}`,
                    "X-Chunk-Sha256": chunkSha,
                    Authorization: token,
                },
                body: chunk,
            },
        );
        if (!resp.ok) {
            const payload = await resp.json().catch(() => ({}));
            throw new Error(payload.message ?? resp.status);
        }
    }

    async function runAnalyze() {
        data.isLoading = true;
        data.error = "";
        try {
            data.analysis = await api(`/pocketbase-imports/${data.sessionId}/analyze`, "POST");
            await refreshSession();
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function runPlan() {
        data.isLoading = true;
        data.error = "";
        try {
            const placements = (data.analysis.collections ?? [])
                .filter((c) => c.type === "base" || c.type === "auth")
                .map((c) => ({
                    collectionId: c.name,
                    placement: "shared",
                    ownership: { kind: "shared" },
                    uniquePolicy: "preserve-global",
                }));
            const plan = await api(`/pocketbase-imports/${data.sessionId}/plan`, "PUT", {
                expectedRevision: String(data.sessionRevision),
                sourceHash: data.session.sourceHash,
                snapshotHash: data.session.snapshotHash,
                layout: "shared",
                namespace: "shared",
                tenantKind: null,
                tenantSource: null,
                access: null,
                placements: placements,
                accountPolicy: {
                    sessionPolicy: "preserve-if-compatible",
                    operatorPolicy: "use-source-superuser",
                },
                selector: { kind: "none", source: null },
                profile: "wal",
                sourceFileStoreRef: "",
                settingsKeyRef: "",
                sideEffects: "suppressed-during-import",
            });
            data.planHash = plan.planHash;
            await refreshSession();
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function runPreview() {
        data.isLoading = true;
        data.error = "";
        try {
            data.preview = await api(`/pocketbase-imports/${data.sessionId}/preview`, "POST");
            await refreshSession();
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function runStage() {
        data.isLoading = true;
        data.error = "";
        try {
            data.staged = await api(`/pocketbase-imports/${data.sessionId}/stage`, "POST");
            await refreshSession();
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function runValidate() {
        data.isLoading = true;
        data.error = "";
        try {
            data.validation = await api(`/pocketbase-imports/${data.sessionId}/validate`, "POST");
            await refreshSession();
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function runCutover() {
        data.isLoading = true;
        data.error = "";
        try {
            const reportHash = data.session.reportHash ?? "";
            data.session = await api(`/pocketbase-imports/${data.sessionId}/cutover`, "POST", {
                expectedRevision: String(data.sessionRevision),
                planHash: data.planHash,
                previewHash: data.preview.previewHash,
                reportHash: reportHash,
            });
            data.sessionPhase = data.session.phase;
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    // fieldEl renders the .field box and (optionally) the help text as a
    // sibling — the dashboard puts field-help outside the bordered input box.
    function fieldEl(labelText, control, helpText) {
        return t.div(
            null,
            t.div({ className: "field" }, t.label({}, labelText), control),
            helpText ? t.div({ className: "field-help" }, helpText) : null,
        );
    }

    function infoRow(labelText, value) {
        return t.div(
            { className: "flex gap-10 m-b-xs" },
            t.span({ className: "txt-sm txt-hint", style: "min-width: 130px;" }, labelText),
            t.span({ className: "txt-sm", style: "word-break: break-all;" }, () => value),
        );
    }

    function phaseBadge() {
        if (!data.phase) return "";
        return t.span({ className: "label sm" }, () => `phase: ${data.phase}`);
    }

    function sourceSection() {
        // hidden native input + styled label-button, the same picker pattern
        // used by the collections import and backups upload controls
        const fileInput = t.input({
            id: "mirabase_import_source_file",
            type: "file",
            className: "hidden",
            onchange: (e) => (data.file = e.target?.files?.[0] ?? null),
        });

        return t.div(
            { className: "block m-b-base" },
            t.h5({ className: "txt-bold m-b-sm" }, "Source snapshot"),
            t.div(
                { className: "grid" },
                t.div(
                    { className: "col-lg-8" },
                    fieldEl(
                        "PocketBase data directory snapshot",
                        t.div(
                            { className: "mb-import-file" },
                            t.label(
                                {
                                    htmlFor: fileInput.id,
                                    className: () => `btn sm outline ${data.isLoading ? "loading" : ""}`,
                                },
                                t.i({ className: "ri-folder-upload-line", ariaHidden: true }),
                                t.span({ className: "txt" }, "Choose file"),
                            ),
                            fileInput,
                            t.span(
                                { className: "mb-import-filename txt-sm txt-hint" },
                                () => data.file?.name || "No snapshot selected",
                            ),
                        ),
                        "A stopped PocketBase data.db (or a backup zip of the data directory). The upload is chunked and hash verified; nothing is written to the live application.",
                    ),
                ),
            ),
            t.div(
                { className: "flex gap-10 m-t-base" },
                t.button(
                    {
                        type: "button",
                        className: () => `btn expanded ${data.isLoading ? "loading" : ""}`,
                        disabled: () => data.isLoading || !data.file,
                        onclick: createSession,
                    },
                    t.span({ className: "txt" }, "Create session and upload"),
                ),
            ),
        );
    }

    function sessionSection() {
        if (!data.sessionId) return "";
        return t.div(
            { className: "block m-b-base" },
            t.div(
                { className: "flex gap-10 m-b-sm" },
                t.h5({ className: "txt-bold" }, "Import pipeline"),
                phaseBadge(),
            ),
            infoRow("Session", data.sessionId),
            infoRow("Source sha256", data.session.sourceHash),
            t.hr({ className: "m-b-sm" }),
            stepRow(
                "Analyze the source",
                "Reads the uploaded snapshot read-only and reports integrity, migrations and collections.",
                runAnalyze,
                () => data.canAnalyze,
            ),
            stepRow(
                "Draft the plan",
                "Maps every base/auth collection to the shared layout and freezes the plan hash.",
                runPlan,
                () => data.canPlan,
            ),
            stepRow(
                "Compute the preview",
                "Counts the source, shared and required file rows without touching the target.",
                runPreview,
                () => data.canPreview,
            ),
            stepRow(
                "Stage rows and files",
                "Copies rows, auth hashes and file content into the unpublished target.",
                runStage,
                () => data.canStage,
            ),
            stepRow(
                "Validate",
                "Replays count, hash and auth sign-in cases; cutover refuses to run with unresolved errors.",
                runValidate,
                () => data.canValidate,
            ),
            stepRow(
                "Cut over",
                "Publishes the staged application through the guarded journal. The source and the old target are retained.",
                runCutover,
                () => data.canCutover,
            ),
            t.div(
                { className: "alert success m-t-sm", hidden: () => data.phase !== "active" },
                "The import is active. The staged application is published; the original source and the old target are retained.",
            ),
        );
    }

    function stepRow(titleText, helpText, action, enabled) {
        return t.div(
            { className: "mb-import-step flex gap-10 m-b-xs" },
            t.div(
                { className: "flex-fill m-r-auto" },
                t.div({ className: "txt-sm" }, titleText),
                t.div({ className: "field-help", style: "margin: 0;" }, helpText),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn sm outline secondary",
                    disabled: () => data.isLoading || !enabled(),
                    onclick: action,
                },
                t.span({ className: "txt" }, "Run"),
            ),
        );
    }

    function resultsSection() {
        const rows = [];
        if (data.analysis) {
            const colls = (data.analysis.collections ?? [])
                .map((c) => `${c.name} (${c.type}, ${c.recordCount})`)
                .join(", ");
            rows.push(t.div({ className: "txt-bold txt-sm m-b-xs" }, "Analysis"));
            rows.push(infoRow("Integrity", data.analysis.integrity));
            rows.push(infoRow("Migrations applied", data.analysis.migrationsApplied));
            rows.push(infoRow("Collections", colls || "none"));
        }
        if (data.preview) {
            rows.push(t.hr({ className: "m-t-sm m-b-sm" }));
            rows.push(t.div({ className: "txt-bold txt-sm m-b-xs" }, "Preview"));
            rows.push(infoRow("Source records", data.preview.sourceRecords));
            rows.push(infoRow("Shared records", data.preview.sharedRecords));
            rows.push(infoRow("Required files", data.preview.requiredFiles));
        }
        if (data.validation) {
            rows.push(t.hr({ className: "m-t-sm m-b-sm" }));
            rows.push(t.div({ className: "txt-bold txt-sm m-b-xs" }, "Validation"));
            rows.push(infoRow("Passed", data.validation.passed));
            rows.push(infoRow("Cases", `${data.validation.passedCases} passed / ${data.validation.failedCases} failed`));
        }
        if (rows.length === 0) return "";
        return t.div({ className: "block m-b-base" }, rows);
    }

    return t.div(
        { pbEvent: "pageMirabaseImport", className: "page page-mirabase-import" },
        settingsSidebar(),
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, "Settings"),
                    t.div({ className: "breadcrumb-item" }, () => app.store.title),
                ),
            ),
            t.div(
                { className: "wrapper m-b-base" },
                t.div(
                    { className: "grid" },
                    t.div(
                        { className: "col-lg-12" },
                        t.div(
                            { className: "flex gap-10 m-b-sm" },
                            t.div({ className: "txt-lg" }, "PocketBase shared-layout adoption"),
                        ),
                        t.div(
                            { className: "txt-sm txt-hint m-b-base" },
                            "Import a stopped PocketBase data directory (data.db) into MiraBase with records, auth (bcrypt hashes), rules, migrations and settings preserved. The copy-and-verify pipeline runs with side effects suppressed; the original source and the old target are retained; validation must pass with zero unresolved errors before cutover.",
                        ),
                        t.div(
                            { className: "alert danger m-b-base", hidden: () => !data.error },
                            () => data.error,
                        ),
                        sourceSection(),
                        () => resultsSection(),
                        () => sessionSection(),
                    ),
                ),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
