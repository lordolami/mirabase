# Authentication and MFA

MiraBase auth collections support password authentication, OAuth2 providers,
one-time passwords, refresh tokens, verification flows, password reset, email
change, impersonation by authorized superusers, and multi-factor authentication
when an auth collection enables it.

## Password and token flows

- Authenticate with the collection's configured identity fields and password.
- Refresh tokens rotate according to the collection auth options.
- Password fields are write-only in public responses.
- Hidden and protected fields must not be exposed through record reads, search,
  live queries, or reference-copy projections.

## OAuth2

Configure each provider in the auth collection settings. Redirect URLs must
belong to the application being served. Provider account data is normalized
into the auth record and external-auth relation; provider secrets stay in
protected settings.

## OTP and MFA

One-time passwords and MFA challenges expire according to their collection
settings. Expired challenges must be rejected and cannot be replayed. MFA
methods and secrets are protected data and are never eligible for search or
reference-copy projection.

## Superusers

Application superusers administer that application's collections and settings.
They do not automatically become instance operators or gain PostgreSQL gateway
authority. Gateway access requires separately issued scoped credentials.
