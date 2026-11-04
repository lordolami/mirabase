/**
 * @mirabase/sdk — the owned MiraBase JavaScript/TypeScript client.
 *
 * Entry: forApplication(baseUrl) → immutable withContext({databaseId}) /
 * withTenant({namespace,key}) views. Inherited collection methods remain
 * on the underlying inherited SDK; the additive typed services are
 * delivered alongside their owning gates.
 */
export {
	MirabaseClient,
	MirabaseContextView,
	MirabaseTenantView,
	forApplication,
} from "./context.js";
export type { ContextOptions, TenantOptions, MirabaseFetchOptions } from "./context.js";
