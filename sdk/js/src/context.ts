/**
 * MiraBase context-aware client views (@mirabase/sdk).
 *
 * Context is immutable per scoped client instance
 * (api-and-sdk.md): "simultaneous calls cannot race through a shared
 * currentDatabase property." A scoped view sends the resolved selector on
 * every request via the X-MiraBase-Database / X-MiraBase-Tenant headers;
 * invalid or unauthorized selection never falls back to primary on the
 * server, and the view never rewrites itself after creation.
 *
 * v0.1: one application (default). `forApplication(baseUrl)` is the
 * immutable entry shape; `/a/<applicationId>` remains the reserved
 * immutable SDK base prefix for later applications.
 */

export type ContextOptions = { databaseId: string };
export type TenantOptions = { namespace: string; key: string };

export interface MirabaseFetchOptions extends RequestInit {
	query?: Record<string, string>;
}

/** Low-level transport shared by all views. */
export class MirabaseClient {
	readonly baseUrl: string;
	private tokenProvider: () => string | undefined;

	constructor(baseUrl: string, tokenProvider?: () => string | undefined) {
		this.baseUrl = baseUrl.replace(/\/+$/, "");
		this.tokenProvider = tokenProvider ?? (() => undefined);
	}

	/** withContext returns an immutable database-scoped view. */
	withContext(options: ContextOptions): MirabaseContextView {
		return new MirabaseContextView(this, { databaseId: options.databaseId });
	}

	/** withTenant returns an immutable tenant-scoped view. */
	withTenant(options: TenantOptions): MirabaseTenantView {
		return new MirabaseTenantView(this, { namespace: options.namespace, key: options.key });
	}

	setTokenProvider(provider: () => string | undefined): void {
		this.tokenProvider = provider;
	}

	async request(
		path: string,
		options: MirabaseFetchOptions = {},
		extraHeaders: Record<string, string> = {},
	): Promise<any> {
		if (!path.startsWith("/") || path.startsWith("//")) {
			throw new Error("request path must be server-relative and begin with a single slash");
		}
		const url = new URL(this.baseUrl + path);
		const base = new URL(this.baseUrl);
		const prefix = base.pathname.replace(/\/+$/, "");
		if (url.origin !== base.origin || (prefix && !url.pathname.startsWith(prefix + "/"))) {
			throw new Error("request path escaped its application URL");
		}
		if (options.query) {
			for (const [k, v] of Object.entries(options.query)) {
				url.searchParams.set(k, v);
			}
		}
		const headers = new Headers(options.headers);
		if (!headers.has("Accept")) headers.set("Accept", "application/json");
		// The immutable view owns its selectors, including when callers supply
		// a Headers instance or tuple array rather than a plain object.
		for (const [key, value] of Object.entries(extraHeaders)) headers.set(key, value);
		const token = this.tokenProvider();
		if (token) {
			headers.set("Authorization", token);
		}
		if (typeof options.body === "string" && !headers.has("Content-Type")) {
			headers.set("Content-Type", "application/json");
		}
		const resp = await fetch(url.toString(), { ...options, headers });
		const text = await resp.text();
		let body: any = undefined;
		if (text) {
			try {
				body = JSON.parse(text);
			} catch {
				body = text;
			}
		}
		if (!resp.ok) {
			// Structured error envelope without string parsing:
			// {status, message, data:{mirabase:{code, retryable, ...}}}.
			const code =
				body && body.data && body.data.mirabase && body.data.mirabase.code
					? body.data.mirabase.code
					: "invalid_request";
			const err = new Error(
				typeof body?.message === "string" ? body.message : `request failed (${resp.status})`,
			) as Error & { code: string; status: number; response: any };
			err.code = code;
			err.status = resp.status;
			err.response = body;
			throw err;
		}
		return body;
	}
}

/** Immutable database-scoped view (X-MiraBase-Database). */
export class MirabaseContextView {
	constructor(
		private readonly client: MirabaseClient,
		private readonly options: Readonly<ContextOptions>,
	) {
		Object.freeze(this.options);
		Object.freeze(this);
	}

	get databaseId(): string {
		return this.options.databaseId;
	}

	async request(path: string, options: MirabaseFetchOptions = {}): Promise<any> {
		return this.client.request(path, options, {
			"X-Mirabase-Database": this.options.databaseId,
		});
	}

	/** Context-bound record CRUD on /api/mirabase/v1. */
	async listRecords(collection: string, limit = 100): Promise<any> {
		return this.request(
			`/api/mirabase/v1/db/${encodeURIComponent(this.options.databaseId)}/collections/${encodeURIComponent(collection)}/records`,
			{ method: "GET", query: { limit: String(limit) } },
		);
	}

	async getRecord(collection: string, recordId: string): Promise<any> {
		return this.request(
			`/api/mirabase/v1/db/${encodeURIComponent(this.options.databaseId)}/collections/${encodeURIComponent(collection)}/records/${encodeURIComponent(recordId)}`,
			{ method: "GET" },
		);
	}

	async createRecord(collection: string, data: Record<string, unknown>): Promise<any> {
		return this.request(
			`/api/mirabase/v1/db/${encodeURIComponent(this.options.databaseId)}/collections/${encodeURIComponent(collection)}/records`,
			{ method: "POST", body: JSON.stringify(data) },
		);
	}

	async updateRecord(collection: string, recordId: string, data: Record<string, unknown>): Promise<any> {
		return this.request(
			`/api/mirabase/v1/db/${encodeURIComponent(this.options.databaseId)}/collections/${encodeURIComponent(collection)}/records/${encodeURIComponent(recordId)}`,
			{ method: "PATCH", body: JSON.stringify(data) },
		);
	}

	async deleteRecord(collection: string, recordId: string): Promise<void> {
		await this.request(
			`/api/mirabase/v1/db/${encodeURIComponent(this.options.databaseId)}/collections/${encodeURIComponent(collection)}/records/${encodeURIComponent(recordId)}`,
			{ method: "DELETE" },
		);
	}
}

/** Immutable tenant-scoped view (X-MiraBase-Tenant: namespace/key). */
export class MirabaseTenantView {
	constructor(
		private readonly client: MirabaseClient,
		private readonly options: Readonly<TenantOptions>,
	) {
		Object.freeze(this.options);
		Object.freeze(this);
	}

	get namespace(): string {
		return this.options.namespace;
	}

	get key(): string {
		return this.options.key;
	}

	async request(path: string, options: MirabaseFetchOptions = {}): Promise<any> {
		return this.client.request(path, options, {
			"X-Mirabase-Tenant": `${this.options.namespace}/${this.options.key}`,
		});
	}
}

/** forApplication — the immutable entry shape. */
export function forApplication(baseUrl: string, tokenProvider?: () => string | undefined): MirabaseClient {
	return new MirabaseClient(baseUrl, tokenProvider);
}
