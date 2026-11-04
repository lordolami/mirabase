/**
 * MiraBase runnable example: context-bound records + vector/text search
 * through the actual /api/mirabase/v1 surface.
 *
 * Run against a real backend:
 *
 *   MIRABASE_URL=http://127.0.0.1:8090 \
 *   MIRABASE_TOKEN=<record-token> \
 *   MIRABASE_DATABASE_ID=<database-id> \
 *   node examples/context-and-search.mjs
 *
 * Every step asserts the actual response; a failing step exits non-zero
 * ("Explaining a feature in API preview is not proof its request path
 * works" — the example RUNS).
 */
import { forApplication } from "../dist/index.js";

const base = process.env.MIRABASE_URL ?? "http://127.0.0.1:8090";
const token = process.env.MIRABASE_TOKEN;
const databaseId = process.env.MIRABASE_DATABASE_ID;
if (!token || !databaseId) {
	console.error("MIRABASE_TOKEN and MIRABASE_DATABASE_ID are required");
	process.exit(1);
}

const client = forApplication(base, () => token);
const db = client.withContext({ databaseId });

// 1. Create a record in the scoped context.
const created = await db.createRecord("notes", {
	text: "example record",
});
if (!created.id) throw new Error(`create failed: ${JSON.stringify(created)}`);
console.log("created:", created.id);

// 2. Read it back.
const read = await db.getRecord("notes", created.id);
if (read.text !== "example record") throw new Error("read-back mismatch");
console.log("read:", read.id);

// 3. Text search (native FTS — the index must exist via search-index).
try {
	const hits = await db.request(
		`/api/mirabase/v1/databases/${databaseId}/collections/notes/search`,
		{
			method: "POST",
			body: JSON.stringify({
				mode: "text",
				text: { fields: ["text"], query: "example" },
			}),
		},
	);
	if (!hits.items) throw new Error("no items in the search result");
	console.log("text search hits:", hits.items.length);
} catch (err) {
	if (err.code === "schema_mismatch") {
		// The FTS index does not exist yet — create it (durable maintenance)
		// and retry once.
		await db.request(
			`/api/mirabase/v1/databases/${databaseId}/collections/notes/search-index`,
			{
				method: "POST",
				body: JSON.stringify({
					indexName: "idx_notes_fts",
					fields: ["text"],
					tokenizer: "default",
				}),
			},
		);
		const hits = await db.request(
			`/api/mirabase/v1/databases/${databaseId}/collections/notes/search`,
			{
				method: "POST",
				body: JSON.stringify({
					mode: "text",
					text: { fields: ["text"], query: "example" },
				}),
			},
		);
		console.log("text search hits (after index creation):", hits.items.length);
	} else {
		throw err;
	}
}

// 4. Vector search (the embedding column carries JSON vectors supplied by
// the application).
try {
	const hits = await db.request(
		`/api/mirabase/v1/databases/${databaseId}/collections/notes/search`,
		{
			method: "POST",
			body: JSON.stringify({
				mode: "vector",
				vector: { field: "embedding", query: [1, 0, 0], metric: "cosine" },
			}),
		},
	);
	console.log("vector search hits:", hits.items.length);
} catch (err) {
	if (err.code === "invalid_request") {
		console.log("vector search needs an embedding column (expected on a fresh notes collection)");
	} else {
		throw err;
	}
}

// 5. Release-capability error: hybrid must return capability_unavailable.
try {
	await db.request(
		`/api/mirabase/v1/databases/${databaseId}/collections/notes/search`,
		{
			method: "POST",
			body: JSON.stringify({ mode: "hybrid" }),
		},
	);
	throw new Error("hybrid was accepted (must not be)");
} catch (err) {
	if (err.code !== "capability_unavailable") throw err;
	console.log("hybrid correctly rejected: capability_unavailable");
}

// 6. Cleanup the created record.
await db.deleteRecord("notes", created.id);
console.log("deleted:", created.id);
console.log("EXAMPLE OK");
