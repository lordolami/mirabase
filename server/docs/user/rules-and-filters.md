# Rules and filters

Collection API rules control list, view, create, update, and delete access.
Rules are evaluated for the current request actor and record context before
records are returned or changed.

## Filter syntax

Filters can compare record fields with literal values and supported request or
collection values. Typical operators include equality, inequality, relational
comparisons, text matching, collection membership, and logical grouping.

Examples:

```text
status = "published"
author = @request.auth.id
publishedAt <= @now
title ~ "release"
```

The exact fields available depend on the collection and request context.
Dashboard rule editors show the valid fields for the current collection.

## Safety rules

- Ordinary record rules never authorize privileged SQL or PostgreSQL gateway
  access.
- Search and live queries apply the same collection rule boundary before data
  leaves the server.
- Hidden and protected fields are removed from ordinary responses.
- Filters supplied by clients are parsed and parameterized; they are not
  concatenated into SQL as raw fragments.
