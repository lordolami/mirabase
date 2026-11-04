package apis

import (
	"net/http"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/setup"
	"mirabase/tools/router"
)

// bindMirabaseCapabilitiesApi registers GET /api/mirabase/v1/capabilities
// (the released operation set, narrowed per the v0.1 allocation): the
// released additive operations, the release boundaries, and the engine
// revision (branding contract: product, engine revision and build
// capabilities reported separately).
func bindMirabaseCapabilitiesApi(app core.App, subGroup *router.RouterGroup[*core.RequestEvent]) {
	subGroup.GET("/capabilities", func(e *core.RequestEvent) error {
		rt, err := application.BindRuntime(app)
		if err != nil {
			return e.InternalServerError("The partitioning runtime is not available.", err)
		}
		released := []string{
			"capabilities",
			"setup.get", "setup.plan", "setup.apply", "setup.administrator",
			"database.list", "database.create", "database.get", "database.profile",
			"database.maintenance", "database.maintain", "database.diagnostics", "database.delete",
			"grant.list", "grant.put", "grant.revoke",
			"agent.bind", "agent.revoke", "agent.list",
			"schema.plan", "schema.apply",
			"template.publish", "template.list", "template.get", "template.rollout",
			"rollout.status", "rollout.run", "rollout.pause", "rollout.resume", "rollout.retry", "rollout.targets",
			"membership.invite", "membership.accept", "membership.revoke",
			"owner.transfer",
			"db.recordList", "db.recordGet", "db.recordCreate", "db.recordUpdate", "db.recordDelete",
			"search", "database.searchIndex",
			"job.get", "job.list", "job.cancel", "receipt.get",
			"backup.create", "backup.restore",
			"reference.define", "reference.status", "reference.refresh",
			"changes.pull", "changes.stream",
			"credential.issue", "credential.revoke", "credential.list",
			"sync.bootstrap", "sync.page", "sync.push", "sync.pull", "sync.ack",
			"sync.file.stage", "sync.file.upload",
			"export", "import", "key.rotate",
			"agentfs.workspace.create", "agentfs.workspace.list", "agentfs.file.write",
			"agentfs.file.read", "agentfs.file.delete", "agentfs.snapshot.create",
			"agentfs.snapshot.restore",
			"application.list", "application.create", "application.get",
			"application.state", "application.delete", "application.bootstrap",
			"layout.apply",
			"importbatch.create", "importbatch.get", "importbatch.sources",
			"importbatch.run", "importbatch.cancel",
			"pbimport.create", "pbimport.source.upload", "pbimport.source.get",
			"pbimport.analyze", "pbimport.inventory", "pbimport.plan.put", "pbimport.plan.get",
			"pbimport.preview.compute", "pbimport.preview.get", "pbimport.tenants", "pbimport.issues",
			"pbimport.stage", "pbimport.validate", "pbimport.report", "pbimport.report.cases",
			"pbimport.resume", "pbimport.recovery", "pbimport.artifact.get", "pbimport.cutover",
			"pbimport.cancel", "pbimport.get",
			"liveplan.list", "liveplan.create", "liveplan.get", "liveplan.publish", "liveplan.retire",
			"live.register", "live.stream", "live.cancel",
			"relationworkflow.define", "relationworkflow.get", "relationworkflow.run",
			"report.define", "report.get", "report.rebuild",
		}
		unregistered := []string{
			// The operations http-operation-reference.md retains for the
			// roadmap and that are genuinely not implemented (and not
			// registered) in v0.1: the instance-scoped application
			// maintenance family and the per-application import-batch
			// source page (roadsmap.md ownership). Everything else in the
			// operation reference is a bound, released operation above —
			// the list must never name an implemented operation.
			"application.update", "application.backup", "application.restore",
			"importbatch.source.list",
		}
		svc, svcErr := setup.NewService(rt.PrimaryDB, rt)
		layout := "shared"
		if svcErr == nil {
			layout = svc.ActiveLayout(e.Request.Context())
		}
		return e.JSON(http.StatusOK, map[string]any{
			"product":             "MiraBase",
			"applicationId":       application.ApplicationID,
			"layout":              layout,
			"engineRevision":      "b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd",
			"patchSetDigest":      "none",
			"apiBase":             "/api/mirabase/v1",
			"released":            released,
			"unregisteredIn_v0.1": unregistered,
			"searchModes":         []string{"vector", "text", "hybrid", "fuzzy"},
			"granularScopes":      []string{"record.read", "record.write", "files.read", "files.write", "database.inspect"},
			"layouts":             []string{"shared", "user", "tenant", "agent"},
			"profiles":            []string{"wal", "mvcc"},
		})
	})
}
