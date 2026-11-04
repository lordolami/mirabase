# SDKs and programming languages

Clients connect over HTTP; they do not contain the full backend. The Go framework embeds a trusted server. Native Turso bindings expose an engine, not MiraBase auth/files/collections.

## Choose the client surface

| Need | Current option | Coverage limit |
|---|---|---|
| Collection/auth/file/realtime API | Individually qualified inherited compatible client or HTTP | Verify against the actual candidate. |
| Explicit managed DB records | Owned JS/TS or Dart helpers | Full inherited rules/forms/hooks/hidden-field parity is unfinished. |
| Search, schemas, jobs and imports | Generic authenticated request or HTTP | Actor/target authority and DTOs differ by route. |
| SQL tools | Qualified PostgreSQL driver | Separate SQL credentials, required TLS and protocol limits. |
| Offline local storage | Planned adapters | Remote wrappers contain no complete local engine/store. |

Never distribute a superuser/SQL token to make an ordinary frontend request work. See [database limits](databases.md).

## JavaScript and TypeScript

The owned `@mirabase/sdk` provides `forApplication(baseUrl, tokenProvider)`, immutable `withContext({databaseId})`/`withTenant({namespace,key})`, generic `request`, and managed-record list/get/create/update/delete helpers. It is not a complete inherited auth/files/realtime client or an offline local engine.

```sh
cd sdk/js
npm ci
npm run build
npm pack
```

```js
import { forApplication } from '@mirabase/sdk';
const api = forApplication('https://your-backend.example', () => sessionToken);
const db = api.withContext({ databaseId });
const rows = await db.listRecords('notes');
```

Obtain `sessionToken` and `databaseId` through your own authenticated authorized workflow. The example does not provide credentials. ESM JavaScript/types are built to `dist/`; a local archive can be installed without registry publication. Generic request paths include `/api/...` relative to the server URL. Immutable views own selector headers.

Use a qualified inherited compatible client for collection/auth/files/realtime methods the wrapper does not implement. Do not claim this wrapper has `authStore` or all inherited methods. [Actual JS API](../../sdk/js/src/context.ts) · [example](../../sdk/js/examples/context-and-search.mjs).

### Owned JS methods

| Method | Behavior |
|---|---|
| `forApplication(baseUrl, tokenProvider)` | Create transport for the actual URL and identity. |
| `withContext({databaseId})` | Immutable database view. |
| `withTenant({namespace,key})` | Immutable tenant selector; not a credential. |
| `request(path, options)` | Server-relative generic request. |
| `listRecords(collection, limit)` | Managed list, not inherited filter/sort/expand paging. |
| `getRecord(collection, recordId)` | Managed record read. |
| `createRecord(collection, data)` | JSON managed create. |
| `updateRecord(collection, recordId, data)` | JSON managed patch. |
| `deleteRecord(collection, recordId)` | Managed delete. |

The database view has managed-record helpers; the tenant view has scoped generic requests. Request paths stay inside the application URL. Immutable view selectors override supplied selector headers, including Headers/tuple-array forms. Errors expose actual status/code/response; do not parse human messages for authority.

### Concurrent contexts

This controlled integration example assumes one identity already authorized for both targets. Managed CRUD's policy parity remains unfinished.

```js
import { forApplication } from '@mirabase/sdk';

export async function readTwoDatabases(baseUrl, tokenProvider, firstId, secondId) {
  const api = forApplication(baseUrl, tokenProvider);
  const first = api.withContext({ databaseId: firstId });
  const second = api.withContext({ databaseId: secondId });
  return Promise.all([
    first.listRecords('notes', 20),
    second.listRecords('notes', 20),
  ]);
}
```

Do not mutate a global selected database between pending calls or change accounts while unresolved work still belongs to the old identity.

## Dart

The `mirabase` source/path package provides context/tenant views, generic requests and managed-record helpers. It imports `dart:io`; Flutter Web compatibility is not established. Qualify with `dart analyze`, package tests and actual server traffic on each advertised runtime. No complete offline Turso store is bundled.

Use the supplied path source or a separately published artifact. [Dart source](../../sdk/dart/lib/src/context.dart) · [example](../../sdk/dart/example/context_and_search.dart).

## Other languages

Use authenticated HTTP/streaming or qualified inherited compatible clients. Read OpenAPI together with the [bound route reference](api-reference.md); schema completeness is a gate. Never invent equal SDK coverage or registry publication across languages.
