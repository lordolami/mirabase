package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Context header names (partitioning.md): "X-MiraBase-Database and the
// reviewed X-MiraBase-Tenant selector follow [partitioning]. Invalid or
// unauthorized selection never falls back to primary."
const (
	HeaderDatabase = "X-Mirabase-Database"
	HeaderTenant   = "X-Mirabase-Tenant"
)

// Context is the immutable application context (application-isolation.md):
// "Use the immutable default ApplicationContext with ID default from the
// beginning. Resolve the database and current authority before
// rules/forms/hooks/files/SSE/search."
type Context struct {
	ApplicationID string
	// DatabaseID selects a managed target; empty means the primary
	// database (COALESCE(database_id,'primary') semantics).
	DatabaseID string
	// TenantKey is the reviewed tenant selector value (namespace/key);
	// present only for tenant layouts.
	TenantNamespace string
	TenantKey       string
	// PurposeBoundToken carries the context token for files/SSE when
	// headers are unavailable ("Files and SSE use purpose-bound context
	// tokens when headers are unavailable").
	PurposeBoundToken string
}

// IsPrimary reports whether the context targets the primary database.
func (c *Context) IsPrimary() bool { return c.DatabaseID == "" || c.DatabaseID == PrimaryDatabaseID }

// TargetDatabaseID resolves to the registry/database id to operate on.
func (c *Context) TargetDatabaseID() string {
	if c.IsPrimary() {
		return PrimaryDatabaseID
	}
	return c.DatabaseID
}

// String renders the context for diagnostics (never secrets).
func (c *Context) String() string {
	if c.IsPrimary() && c.TenantNamespace == "" {
		return fmt.Sprintf("app=%s db=primary", c.ApplicationID)
	}
	parts := []string{fmt.Sprintf("app=%s", c.ApplicationID)}
	if !c.IsPrimary() {
		parts = append(parts, "db="+c.DatabaseID)
	}
	if c.TenantNamespace != "" {
		parts = append(parts, "tenant="+c.TenantNamespace)
	}
	return strings.Join(parts, " ")
}

// Resolver errors (never silent fallback to primary).
var (
	// ErrUnknownDatabase reports an unknown or stale database selector.
	ErrUnknownDatabase = errors.New("unknown database selector")
	// ErrUnauthorizedDatabase reports a selector the caller is not
	// authorized for; the request must fail, never fall back to primary.
	ErrUnauthorizedDatabase = errors.New("unauthorized database selector")
	// ErrTenantSelection is the multi-eligible-tenant choice requirement.
	ErrTenantSelectionRequired = ErrTenantSelection
)

// type contextKey guards the resolved context value.
type contextKey struct{}

// WithContext attaches the resolved context to a request context.
func WithContext(ctx context.Context, c *Context) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext returns the resolved context; a missing value is the
// immutable default context.
func FromContext(ctx context.Context) *Context {
	if c, ok := ctx.Value(contextKey{}).(*Context); ok && c != nil {
		return c
	}
	return &Context{ApplicationID: ApplicationID}
}

// Resolver turns incoming selector headers into a verified Context. The
// resolution happens before authentication carries no authority by itself:
// authorization is verified against the registry/grants afterwards ("The
// context-bound app exposes the same transaction to reads/writes/hooks";
// "Resolve the database and current authority before
// rules/forms/hooks/files/SSE/search").
type Resolver struct {
	service *Service
}

// NewResolver builds the context resolver over the registry service.
func NewResolver(s *Service) *Resolver { return &Resolver{service: s} }

// ResolveHTTP reads the selector headers and verifies them against the
// registry. Rules:
//   - no selectors → the immutable default (primary) context;
//   - X-MiraBase-Database present → the database must exist AND the caller
//     must be authorized (owner or active grant) — invalid/unauthorized
//     selection NEVER falls back to primary;
//   - contradictory header/path selections are rejected (path ID
//     authoritative when both exist).
func (r *Resolver) ResolveHTTP(req *http.Request, subjectCollectionID, subjectRecordID string) (*Context, error) {
	dbSel := strings.TrimSpace(req.Header.Get(HeaderDatabase))
	tenantSel := strings.TrimSpace(req.Header.Get(HeaderTenant))
	if dbSel == "" && tenantSel == "" {
		return &Context{ApplicationID: ApplicationID}, nil
	}
	if dbSel != "" && tenantSel != "" {
		return nil, fmt.Errorf("choose either a database or a tenant selector, not both")
	}

	if dbSel != "" {
		if dbSel == PrimaryDatabaseID {
			return &Context{ApplicationID: ApplicationID}, nil
		}
		d, err := r.service.GetDatabase(req.Context(), dbSel)
		if errors.Is(err, ErrNotFound) {
			return nil, ErrUnknownDatabase
		}
		if err != nil {
			return nil, err
		}
		if err := r.authorize(req, d, subjectCollectionID, subjectRecordID); err != nil {
			return nil, err
		}
		return &Context{ApplicationID: ApplicationID, DatabaseID: d.ID}, nil
	}

	// Tenant selector: namespace/key; v0.1 requires the namespace to be
	// configured (routes are roadmap-narrowed to the binding table).
	ns, key, _ := strings.Cut(tenantSel, "/")
	if ns == "" || key == "" {
		return nil, fmt.Errorf("the tenant selector must be namespace/key")
	}
	return &Context{ApplicationID: ApplicationID, TenantNamespace: ns, TenantKey: key}, nil
}

// authorize verifies the caller owns the database or holds a live grant.
func (r *Resolver) authorize(req *http.Request, d *Database, subjectCollectionID, subjectRecordID string) error {
	// The owner of the binding is always authorized.
	if d.OwnerCollectionID != nil && *d.OwnerCollectionID == subjectCollectionID &&
		d.OwnerRecordID != nil && *d.OwnerRecordID == subjectRecordID {
		return nil
	}
	// A live grant for this subject authorizes the path.
	g, err := r.service.GetGrant(req.Context(), d.ID, subjectCollectionID, subjectRecordID)
	if errors.Is(err, ErrNotFound) {
		return ErrUnauthorizedDatabase
	}
	if err != nil {
		return err
	}
	if !g.HasScope(ScopeDatabaseInspect) && !g.HasScope(ScopeRecordRead) && !g.HasScope(ScopeRecordWrite) {
		return ErrUnauthorizedDatabase
	}
	return nil
}
