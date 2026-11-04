# Building a new application

**Current release authority:** [current implementation inventory](../current-product.md), [user documentation](../user/index.md) and [launch status](../launch/status.md). Initial v0.1/roadmap delivery labels below preserve planning contracts; they do not establish today's implementation or permit deleting subsequently integrated features. [Open-source platform roadmap](../roadmap/platform/README.md) has separate design and gates.

**Delivery: v0.1 and roadmap.** [v0.1-build.md](v0.1-build.md) selects the current portion; [roadsmap.md](roadsmap.md) retains the remainder. Detailed schemas/configuration/ABI and full-family acceptance below describe the target superset unless narrowed by that allocation.

## Required journey

The complete product supports developers starting with no PocketBase source. The setup service, dashboard, CLI and SDK use the same contracts. v0.1 includes complete guided setup for shared/default storage and new empty user/tenant/agent layouts. Populated-data transitions, independent-app bootstrap and built-in invitations follow the roadmap allocation. Ordinary shared-database startup retains the inherited setup and first-superuser experience.

Run `mirabase serve`, open the supplied setup link, establish the first superuser and configure the backend in the existing dashboard. No cloud account, billing object or mandatory tenancy is created. The dashboard's application settings offer the storage choices below and generate examples from the published configuration. Adding an independent application is a server-operator action using the application service, not an ordinary membership action.

| Storage choice | Explanation shown | Source of database identity |
|---|---|---|
| Shared database | Application records share the application's primary database | Reserved application-local `primary` |
| Database per user | Each selected account has private application storage | Auth collection ID + account ID + template, inside this application |
| Database per tenant | Each school/company/workspace shares a database among authorized members | Reviewed tenant collection/key + namespace + template |
| Database per agent | A bound agent has private application storage under its own scopes | Agent auth identity + binding + template |

A tenant can contain thousands of users. Creating or joining a user does not necessarily create a tenant. An independent imported application is a tenant/application boundary; it is not automatically split by accounts. Display a diagram and actual planned counts before applying partitioning. Never infer a choice from account count or a field named `userId`.

## Draft, validate and apply

`server/internal/setup.Service` owns storage policies. `GET setup` returns the current revision and any caller-owned draft. `PUT setup/plan` takes expected application generation/revision, storage layout, auth collection IDs, template, collection placements, provisioning policy and tenant/membership bindings. It returns a canonical immutable plan hash, proposed schema, compatibility actions, preview counts and errors. `POST setup/plan/apply` requires the exact plan hash/revision plus idempotency key. The returned durable job owns schema/policy publication. UI states are draft, validating, invalid, ready, applying, active, failed and cancelled-before-publication.

The default is shared layout; optional partition provisioning defaults to lazy. First publish the minimal app-local optional metadata transactionally, then candidate templates/policies and finally the active revision. A cross-file layout change uses the recoverable migration job in implementation-closure.md, never a boolean toggle over existing rows. A failed apply leaves the previous policy active. If any dependent schema or policy changed after planning, return `schema_mismatch` or `policy_changed` before effects.

Every base/auth/view collection receives a placement. Accounts, membership and sensitive settings remain global **within this application**, not shared across unrelated applications. Private records use the selected database. Read-only reference copies and derived views require the declared refresh/dependency contract. The planner rejects ambiguous owners, invalid rule translation, hidden cross-file joins/cascades, changed uniqueness semantics and unsupported engine/profile combinations before publication. Existing application data requires actual row/file preview and verification; an empty-app plan still checks schema and request fixtures.

## User-owned storage

Select the auth collection and immutable template. The provisioning key is `(applicationId, authCollectionId, authRecordId, templateId)`. Lazy provisioning runs on first authorized private access; eager provisioning enqueues after account commit. A unique binding plus durable create job makes repeated requests converge on one DB. An account rollback creates no DB. Failures retain a visible provisioning job and can be retried; requests receive `provisioning`/retry information rather than being routed to primary.

Registration keeps existing auth APIs/forms and record rules. Clients do not receive superuser database.create authority. The internal provisioner acts on a verified committed identity and reviewed policy. Deleting an account applies the chosen quarantine/retain/delete policy; ownership transfer is an explicit privileged operation. Different auth collections with the same record ID cannot collide.

## Tenant-owned storage and membership

Select a base collection representing tenants or a typed field key, an immutable namespace and the template. A normal example uses `schools`, `users`, and `memberships` with tenant and subject relations. The setup stores field IDs and auth collection IDs, not guessed names. Membership predicates can use an existing membership collection, tenant owner relation or reviewed tenant rule. Existing role fields remain application-defined; MiraBase does not require a second global role system.

Creating a tenant uses ordinary record CRUD plus the reviewed create rule. Provisioning binds the committed tenant key to one DB, lazily or eagerly. Creating a membership authorizes a user for that existing tenant; it does not provision another DB. Current membership is evaluated on every request/stream boundary. Self-service clients cannot assign another user's membership or elevate role fields unless the existing rules explicitly allow it. Tenant creators can be granted membership only through the configured trusted post-commit policy, not through a client-supplied owner value.

The optional built-in invitation action uses the selected membership fields and existing mail templates. An inviter must pass the configured current-tenant invite rule. Store only a random token hash, application/tenant identity, intended normalized email or existing subject, permitted membership field values, expiry and used/revoked status. Default expiry is seven days. Acceptance requires the authenticated intended subject or a matching verified email, plus current tenant/inviter policy; it commits one membership with a uniqueness guard and atomically consumes the token. Retries return the prior result. Revocation, expiry and email mismatch fail; invitation lookup never exposes other memberships. Account creation/verification continues through inherited auth APIs. Mail is an outbox effect and cannot be resent accidentally by a transaction retry.

An active tenant selector carries the reviewed namespace and typed key. Zero eligible tenants produces an actionable empty state; one can be the default; multiple require a choice. Invalid selection never falls back to primary or scans every tenant. `X-MiraBase-Tenant`/SDK tenant options are resource selectors; membership remains authorization. Browser files/SSE use purpose-bound tokens carrying the resolved application/database/generation.

## Agent-owned storage

Agent registration reuses the agent binding/grant service. An owner can register an agent only under the reviewed application policy. The agent uses its own account and scopes and never the owner's session. Provisioning binds the committed agent identity/template. Revocation advances policy epochs, closes streams and prevents new sync/SQL access as specified; it does not erase data already downloaded by an offline client.

## Frontend and SDK contract

Generate runnable JS/TS and Dart examples from actual collection IDs, base URL and policy revision. Each example demonstrates account creation/login, creating a tenant if permitted, inviting/joining, listing memberships, choosing a tenant and normal collection/file/realtime use. API preview identifies necessary selector changes. Other languages receive exact HTTP/OpenAPI examples rather than fictional full SDK parity.

JS clients use `client.forApplication(baseUrl)` and immutable `client.withContext({databaseId})` or `client.withTenant({namespace,key})` views; Dart exposes equivalent immutable scoped clients. Existing collection methods remain. Auth stores are namespaced by server origin and application ID; explicit client instances can share an auth store only inside the same application. Concurrent requests never mutate a shared selectedTenant/currentDatabase variable. Switching context cancels scoped requests/subscriptions and starts new ones with the new immutable view; pending offline edits remain attached to their original subject/context.

Generated examples show `tenant_selection_required`, provisioning/retry, expired credentials, removed membership, schema mismatch, interrupted upload and server maintenance. They must run against the actual backend. Explaining a feature in API preview is not proof its request path works.

## Acceptance

MB-ONBOARD and MB-MEMBERSHIP run from empty application roots for every layout, through dashboard and HTTP/SDK. Alice/Bob user mode creates exactly two private DBs after first access; two users joining one school creates exactly one school DB; multi-school users must choose; equal IDs in unrelated apps cannot collide. Verify signup rollback, concurrent provisioning, invitation replay/expiry/revocation, forbidden membership edits, owner deletion, files/SSE, operator recovery and restart at every setup publication boundary. Basic shared startup must create no optional instance or partition catalog.


## Initial app and invitation revisions

Application creation starts in shared layout with the inherited bootstrap. Create auth/collection schemas normally, then validate/apply the selected partition policy. GET setup returns SetupState with a nullable draft; it does not invent a plan or install partition metadata. SetupPlanRequest explicitly carries the namespace and all bindings. Invitations have a positive revision; every accept/revoke increments it and stale management requests fail. Token consumption and membership uniqueness are checked in one application-primary transaction.


## Worked request shapes

The payloads below illustrate the target API. IDs/hashes are demonstration values; live previews use actual discovered IDs/revisions. They are not executed migration or runtime results.

### User storage setup

```json
{
  "expectedApplicationGeneration": "1",
  "expectedRevision": "1",
  "layout": "user",
  "namespace": "personal",
  "authCollectionIds": [
    "users"
  ],
  "templateId": "personal-template",
  "provision": "lazy",
  "tenantSource": null,
  "access": null,
  "placements": [
    {
      "collectionId": "users",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "notes",
      "placement": "tenant",
      "ownership": {
        "kind": "field",
        "fieldId": "owner"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "per-tenant-approved",
      "adaptationIds": []
    }
  ]
}
```

### Tenant storage setup

```json
{
  "expectedApplicationGeneration": "1",
  "expectedRevision": "1",
  "layout": "tenant",
  "namespace": "schools",
  "authCollectionIds": [
    "users"
  ],
  "templateId": "school-template",
  "provision": "lazy",
  "tenantSource": {
    "kind": "records",
    "collectionId": "organizations"
  },
  "access": {
    "membership": {
      "kind": "membership-collection",
      "collectionId": "memberships",
      "subjectRelationFieldId": "fld_user",
      "tenantFieldId": "fld_organization"
    },
    "membershipScopes": [
      "record.read",
      "record.write",
      "files.read",
      "files.write",
      "changes.read",
      "record.sync"
    ],
    "allowAnonymousTenantRead": false,
    "ruleAdaptationIds": []
  },
  "placements": [
    {
      "collectionId": "organizations",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "users",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "memberships",
      "placement": "shared",
      "ownership": {
        "kind": "shared"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "projects",
      "placement": "tenant",
      "ownership": {
        "kind": "field",
        "fieldId": "fld_organization"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": [
        "global-org-relation"
      ]
    },
    {
      "collectionId": "tasks",
      "placement": "tenant",
      "ownership": {
        "kind": "relation-path",
        "fieldIds": [
          "fld_project",
          "fld_organization"
        ]
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": []
    },
    {
      "collectionId": "documents",
      "placement": "tenant",
      "ownership": {
        "kind": "field",
        "fieldId": "fld_organization"
      },
      "referenceFieldIds": [],
      "uniquePolicy": "preserve-global",
      "adaptationIds": [
        "global-org-relation"
      ]
    }
  ]
}
```
