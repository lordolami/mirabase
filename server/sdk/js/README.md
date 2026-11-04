# MiraBase JavaScript / TypeScript context client

The package provides immutable database/tenant context views and remote managed-record helpers for MiraBase. It does not contain the server or a local offline database.

## Install the release tarball

Download `mirabase-sdk-0.1.0.tgz` from [MiraBase Releases](https://github.com/lordolami/mirabase/releases), then run in your frontend project:

```sh
npm install ./mirabase-sdk-0.1.0.tgz
```

## Connect

```js
import { forApplication } from '@mirabase/sdk';

export function notesClient(baseUrl, tokenProvider, databaseId) {
  return forApplication(baseUrl, tokenProvider).withContext({ databaseId });
}
// Use your actual authenticated identity and authorized database ID:
// const notes = notesClient(url, () => ordinarySessionToken, databaseId);
// const result = await notes.listRecords('notes', 20);
```

`tokenProvider` supplies an existing permitted session. This wrapper does not implement an authStore, password login, complete file/realtime methods or offline storage. Use ordinary record authentication/HTTP or a qualified inherited compatible client for those operations. Never supply a superuser/SQL token to an untrusted frontend.

## Methods

`forApplication(baseUrl, tokenProvider)` creates the transport. `withContext({databaseId})` returns an immutable database view with `listRecords`, `getRecord`, `createRecord`, `updateRecord`, `deleteRecord` and generic `request`. `withTenant({namespace,key})` returns a scoped generic request view; it does not invent tenant CRUD helpers. Paths are server-relative and stay inside the application URL. Views own selector headers across concurrent calls.

**Current endpoint limitation:** managed-record helpers use `/api/mirabase/v1/db/...`. Database owner/grant checks exist, but complete inherited collection-rule/form/hook/hidden-field parity remains unfinished. That path is distinct from the normal inherited record API. Do not assume inherited filters/expansion/multipart/pagination support.

See the [full SDK guide](https://github.com/lordolami/mirabase/blob/main/docs/user/sdks.md), [API reference](https://github.com/lordolami/mirabase/blob/main/docs/user/api-reference.md), [working inherited-record example](https://github.com/lordolami/mirabase/tree/main/examples/quickstart) and [roadmap](https://github.com/lordolami/mirabase/blob/main/ROADMAP.md). This tarball is a GitHub download, not an assertion of public npm-registry publication.
