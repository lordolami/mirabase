/**
 * MiraBase runnable example: guided setup + template publish + rollout
 * + shared-layout PocketBase adoption through the actual /api/mirabase/v1
 * surface (superuser operations).
 *
 * Run against a real backend:
 *
 *   MIRABASE_URL=http://127.0.0.1:8090 \
 *   MIRABASE_TOKEN=<superuser-token> \
 *   MIRABASE_IMPORT_FILE=./pb_backup/data.db \
 *   node examples/setup-and-adopt.mjs
 */
import { forApplication } from "../dist/index.js";
import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";

const base = process.env.MIRABASE_URL ?? "http://127.0.0.1:8090";
const token = process.env.MIRABASE_TOKEN;
if (!token) {
	console.error("MIRABASE_TOKEN is required");
	process.exit(1);
}
const client = forApplication(base, () => token);
const admin = {
	request(path, options = {}) {
		return client.request(`/api/mirabase/v1${path}`, options);
	},
	async json(path, method, body) {
		return this.request(path, {
			method,
			body: body == null ? undefined : JSON.stringify(body),
		});
	},
};

// 1. Capabilities: the released operations and the engine revision.
const caps = await admin.json("/capabilities", "GET");
console.log("engine revision:", caps.engineRevision);
console.log("released operations:", caps.released.length);

// 2. OpenAPI: the generated document exists and is parseable.
const doc = await admin.json("/openapi.json", "GET");
if (doc.openapi !== "3.1.0") throw new Error("unexpected OpenAPI version");
console.log("openapi paths:", Object.keys(doc.paths).length);

// 3. Setup state (shared default, nullable draft).
const state = await admin.json("/setup", "GET");
console.log("setup layout:", state.layout, "revision:", state.revision);

// 4. Template publish (requires at least one collection on primary).
const templates = await admin.json("/templates", "GET");
console.log("published templates:", templates.totalItems);

// 5. Shared-layout adoption (requires MIRABASE_IMPORT_FILE).
const importFile = process.env.MIRABASE_IMPORT_FILE;
if (importFile) {
	const bytes = await readFile(importFile);
	const sha = createHash("sha256").update(bytes).digest("hex");
	const session = await admin.json("/pocketbase-imports", "POST", {
		sourceKind: "pocketbase-directory",
		sourceBytes: bytes.length,
		sourceSha256: sha,
		targetMode: "new-application",
		expectedPrimaryGeneration: "1",
		sourceLabel: "example adoption",
	});
	console.log("import session:", session.id, session.phase);

	// Chunked upload (single chunk: within the 8 MiB boundary).
	const chunkSha = createHash("sha256").update(bytes).digest("hex");
	const uploaded = await client.request(
		`/api/mirabase/v1/pocketbase-imports/${session.id}/source`,
		{
			method: "PUT",
			headers: {
				"Content-Type": "application/octet-stream",
				"Content-Range": `bytes 0-${bytes.length - 1}/${bytes.length}`,
				"X-Chunk-Sha256": chunkSha,
			},
			body: bytes,
		},
	);
	console.log("upload complete:", uploaded.complete);

	const analysis = await admin.json(
		`/pocketbase-imports/${session.id}/analyze`, "POST");
	console.log("integrity:", analysis.integrity);

	// The plan shapes depend on the source inventory; the example stops
	// here and prints the inventory so the operator continues through the
	// dashboard flow with the actual collections.
	console.log("collections:", analysis.collections.map((c) => c.name));
	console.log("ADOPTION EXAMPLE STAGE OK (continue via /plan → /preview → /stage → /validate → /cutover)");
} else {
	console.log("MIRABASE_IMPORT_FILE not set — skipping the adoption stage");
}
console.log("EXAMPLE OK");
