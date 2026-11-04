# User, tenant and agent databases

A user is an account. A tenant is a company, school or workspace with members. An agent is a separately authenticated identity with bindings/grants. A tenant can have many users and a user can belong to several tenants. These storage choices do not require a process per DB.

## Choose the unit of ownership

| Your application | Layout to evaluate | What shares a database |
|---|---|---|
| Common application records | Shared | Accounts authorized by collection rules. |
| Personal notebooks | User | One account's private context. |
| Company project management | Tenant | Members of that company/workspace. |
| Separately identified research agents | Agent | The bound agent context. |

For example, Alice and Bob can each have user databases while Northstar's employees share one tenant database. Creating a database per employee is a different layout from creating a database per company. Membership in two companies requires a selected company context; an email address or display name is not sufficient authority.

One process manages the files. It does not create a backend process or warm JavaScript VM for every owner. Unique indexes and same-file relations apply to that file; global uniqueness and cross-file transactions do not appear automatically.

## New-application workflow

1. Create the auth/data collections and access policies in [foundation setup](foundation.md).
2. Open the storage-setup screen. Inspect current setup rather than assuming the application is empty.
3. Choose the layout, actual auth collections, provisioning policy and published template. For tenants, configure the tenant source and membership/owner relationships.
4. Draft the setup plan, inspect its proposed databases and validation findings, then apply the reviewed revision.
5. For lazy provisioning, exercise the first authorized owner request. For eager provisioning, verify the configured post-creation path. Inspect the resulting binding and physical schema; readiness includes template application.
6. Resolve the authorized database ID or tenant context through your application's workflow and bind it immutably in the client.
7. Check access with unrelated accounts, grant/revoke transitions, expiry and concurrent first access. Passing one operator request does not prove ordinary-user policy parity.

For an existing populated app, use [adoption planning](pocketbase-import.md) or the applicable reviewed layout transformation. The setup screen must not silently redistribute existing records.

## Operator API map

All paths below are relative to `/api/mirabase/v1`; read the [bound API](api-reference.md) for authority, DTOs and generation/revision fields.

| Task | Method/path |
|---|---|
| Inspect/plan/apply setup | `GET /setup`, `PUT /setup/plan`, `POST /setup/plan/apply` |
| Create/list/inspect a DB | `POST /databases`, `GET /databases`, `GET /databases/{id}` |
| Manage grants | `GET /databases/{id}/grants`, `PUT /databases/{id}/grants`, `DELETE /databases/{id}/grants/{grantId}` |
| Bind/list/revoke an agent | `POST /agents`, `GET /agents`, `DELETE /agents/{agentId}` |
| Inspect resources | `GET /databases/{id}/diagnostics` |
| Maintenance/profile | `POST /databases/{id}/maintenance`, `POST /databases/{id}/actions`, `POST /databases/{id}/profile` |
| Transfer owner | `POST /databases/{id}/owner-transfer` |
| Delete | `DELETE /databases/{id}` |

For a local operator experiment, the minimal manual database request body is:

```json
{
  "displayName": "Operator test database",
  "kind": "manual",
  "profile": "wal"
}
```

A manual DB is ownerless; use the correct user/tenant/agent kind and existing subject binding for private ownership. An empty manual DB does not contain application collections until the appropriate template/schema is applied. Creation returns job information: inspect actual terminal state and the registry, not just the HTTP acknowledgement. Do not add `encryption` or `codecId` expecting protection; those current requests are refused.

## Managed-record boundary

**Current release blocker:** the managed CRUD path calls a raw record store after owner/grant authorization. Complete inherited rules, forms, hooks and hidden-field behavior are not applied. The SDK helpers use this path. Treat existing owner/grant tests as evidence for that boundary only, not as production acceptance for complete per-user backend behavior.

The managed list route currently consumes `limit` and returns `items`, `database`, `page` and `perPage`; it is not the inherited filter/sort/expand pagination implementation. Read individual endpoints before relying on response semantics.

Start shared. Setup selects `shared|user|tenant|agent`, actual auth collections, template and provisioning policy. Tenant layout declares tenant source and membership/owner relationships. Preview and validate before applying. Do not infer owners from field names or silently repartition populated data with a toggle.

Lazy provisioning creates storage on first authorized access; eager provisioning requests it after committed identity/tenant creation. Concurrent requests must converge on one binding. Joining a tenant reuses its DB. Multiple eligible memberships require an explicit choice.

Managed record routes are `/api/mirabase/v1/db/{databaseId}/collections/{collection}/records` and `/{recordId}`. SDK record helpers call these paths. Their managed-store behavior differs from the full inherited record endpoint; inspect [actual contracts](api-reference.md) instead of assuming identical expand/filter/upload coverage.

Inherited context selection uses `X-MiraBase-Database`; reviewed tenant policies use `X-MiraBase-Tenant`. Current owner/membership/grants and rules still apply. Bad selection cannot fall back to primary. Keep context immutable per request and scope files/SSE tokens equivalently.

Operators manage lifecycle, grants, profile, diagnostics and owner transfer. Ordinary users cannot list all DBs, self-grant authority or choose host paths. Restore changes generation; owner deletion follows explicit quarantine/retention/deletion policy.

Same-file relations and uniqueness retain that scope. Private unique indexes are not global uniqueness. References/reports are explicit workflows, not cross-file ACID. Application metadata operations are present, but one tenant DB under shared auth/settings is not automatically an independently isolated application runtime. See [current product](../current-product.md).
