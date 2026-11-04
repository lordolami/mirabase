import { settingsSidebar } from "../settingsSidebar";

// pageMirabaseSetup — the guided storage setup screen (MB-UI slice):
// reads the setup state from /api/mirabase/v1/setup, drafts a user/tenant/
// agent/shared layout plan through PUT /setup/plan, previews the planned
// databases, and applies the validated plan (setup.apply) — the exact
// HTTP surface from V01-06.
export function pageMirabaseSetup(route) {
    app.store.title = "Storage setup";

    const data = store({
        isLoading: false,
        applying: false,
        error: "",
        state: null,
        planForm: {
            layout: "shared",
            authCollectionIds: "",
            templateId: "",
            provision: "lazy",
            namespace: "personal",
            privateCollection: "",
        },
        plan: null,
        planBaseRevision: "",
        applied: false,
        templates: [],
        templateForm: { templateId: "", collectionNames: "" },
        get hasState() {
            return !!data.state;
        },
        get activeLayout() {
            return data.state?.layout ?? "shared";
        },
        get isPrivate() {
            return data.planForm.layout !== "shared";
        },
        get authCollectionsList() {
            return data.planForm.authCollectionIds
                .split(",")
                .map((s) => s.trim())
                .filter((s) => s.length > 0);
        },
        get templateNamesList() {
            return data.templateForm.collectionNames
                .split(",")
                .map((s) => s.trim())
                .filter((s) => s.length > 0);
        },
    });

    loadState();
    loadTemplates();

    async function api(path, method, body) {
        const token = app.pb.authStore.token;
        const headers = { "Content-Type": "application/json" };
        if (token) {
            headers.Authorization = token;
        }
        const resp = await fetch("/api/mirabase/v1" + path, {
            method: method,
            headers: headers,
            body: body === undefined ? undefined : JSON.stringify(body),
        });
        const payload = await resp.json();
        if (!resp.ok) {
            const code = payload?.data?.mirabase?.code ?? "invalid_request";
            throw new Error(code + ": " + (payload.message ?? resp.status));
        }
        return payload;
    }

    async function loadState() {
        data.isLoading = true;
        data.error = "";
        try {
            data.state = await api("/setup", "GET");
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function loadTemplates() {
        try {
            const list = await api("/templates", "GET");
            data.templates = list.items ?? [];
        } catch (err) {
            // the plan flow does not depend on the template listing; keep the
            // page usable and let the plan validation surface any real error
            data.templates = [];
        }
    }

    async function publishTemplate() {
        data.isLoading = true;
        data.error = "";
        try {
            await api("/templates", "POST", {
                templateId: data.templateForm.templateId.trim(),
                collectionNames: data.templateNamesList,
                expectedSchemaRevision: data.state?.revision ?? "",
            });
            app.toasts?.success?.("Template published.");
            data.templateForm.templateId = "";
            data.templateForm.collectionNames = "";
            data.planForm.templateId = "";
            await loadTemplates();
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function draftPlan() {
        data.isLoading = true;
        data.error = "";
        data.plan = null;
        data.applied = false;
        try {
            const privateLayout = data.planForm.layout !== "shared";
            const privateCollection = data.planForm.privateCollection.trim();
            const plan = await api("/setup/plan", "PUT", {
                expectedApplicationGeneration: data.state.applicationGeneration,
                expectedRevision: data.state.revision,
                layout: data.planForm.layout,
                authCollectionIds: privateLayout ? data.authCollectionsList : [],
                templateId: privateLayout ? data.planForm.templateId || null : null,
                provision: "lazy",
                namespace: privateLayout ? data.planForm.namespace : null,
                tenantSource: null,
                access: null,
                placements:
                    privateLayout && privateCollection
                        ? [
                              {
                                  collectionId: privateCollection,
                                  placement: "tenant",
                                  ownership: { kind: "shared" },
                                  referenceFieldIds: [],
                                  uniquePolicy: "preserve-global",
                                  adaptationIds: [],
                              },
                          ]
                        : [],
            });
            data.plan = plan;
            // the apply guard compares the CURRENT schema revision, not the
            // plan row's own revision
            data.planBaseRevision = data.state?.revision ?? "";
        } catch (err) {
            data.error = err.message;
        }
        data.isLoading = false;
    }

    async function applyPlan() {
        if (!data.plan) return;
        data.applying = true;
        data.error = "";
        try {
            await api("/setup/plan/apply", "POST", {
                planId: data.plan.id,
                expectedApplicationGeneration: data.plan.applicationGeneration,
                expectedRevision: String(data.planBaseRevision),
                planHash: data.plan.planHash,
            });
            data.plan = null;
            await loadState();
            // set applied only after the state refresh so the notice renders
            // with the new layout in a single pass
            data.applied = true;
        } catch (err) {
            data.error = err.message;
        }
        data.applying = false;
    }

    // fieldEl renders the .field box and (optionally) the help text as a
    // sibling — the dashboard puts field-help outside the bordered input box.
    function fieldEl(labelText, control, helpText) {
        return t.div(
            null,
            t.div({ className: "field" }, t.label({}, labelText), control),
            helpText ? t.div({ className: "field-help" }, helpText) : null,
        );
    }

    function stateRow(labelText, value) {
        return t.div(
            { className: "flex gap-10 m-b-xs" },
            t.span({ className: "txt-sm txt-hint", style: "min-width: 110px;" }, labelText),
            t.span({ className: "txt-sm", style: "word-break: break-all;" }, () => value),
        );
    }

    function stateSection() {
        return t.div(
            { className: "block m-b-base" },
            t.h5({ className: "txt-bold m-b-sm" }, "Current policy"),
            t.div(
                null,
                () => {
                    if (data.isLoading && !data.hasState) {
                        return t.div({ className: "txt-center m-t-sm" }, t.span({ className: "loader" }));
                    }
                    if (data.error && !data.hasState) {
                        return t.div({ className: "txt-sm txt-danger" }, "The setup state could not be loaded.");
                    }
                    if (!data.hasState) {
                        return "";
                    }
                    return t.div(
                        { className: "block m-b-0" },
                        stateRow("Application", data.state.applicationId),
                        stateRow("Active layout", data.state.layout),
                        stateRow("Generation", data.state.applicationGeneration),
                        stateRow("Revision", data.state.revision),
                    );
                },
            ),
        );
    }

    function templateSection() {
        return t.div(
            { className: "block m-b-base" },
            t.h5({ className: "txt-bold m-b-sm" }, "Templates"),
            t.div(
                { className: "txt-sm txt-hint m-b-sm" },
                "A template is the collection bundle copied into every lazily provisioned database. User, tenant and agent layouts require at least one published template.",
            ),
            t.div(
                { className: "txt-sm m-b-sm" },
                () => {
                    const published = (data.templates ?? [])
                        .map((tpl) => (typeof tpl === "string" ? tpl : tpl?.templateId))
                        .filter(Boolean);
                    if (published.length === 0) {
                        return t.span({ className: "txt-warning" }, "No templates published yet.");
                    }
                    return [t.span({ className: "txt-hint" }, "Published: "), published.join(", ")];
                },
            ),
            t.div(
                { className: "grid" },
                t.div(
                    { className: "col-lg-4" },
                    fieldEl(
                        "New template id",
                        t.input({
                            value: () => data.templateForm.templateId,
                            onchange: (e) => (data.templateForm.templateId = e.target.value),
                            placeholder: "tpl-user",
                        }),
                    ),
                ),
                t.div(
                    { className: "col-lg-8" },
                    fieldEl(
                        "Collections to include",
                        t.input({
                            value: () => data.templateForm.collectionNames,
                            onchange: (e) => (data.templateForm.collectionNames = e.target.value),
                            placeholder: "notes, tasks",
                        }),
                        "Comma separated collection names; each collection is copied empty into the provisioned databases.",
                    ),
                ),
            ),
            t.div(
                { className: "flex gap-10 m-t-base" },
                t.button(
                    {
                        type: "button",
                        className: "btn outline secondary expanded",
                        disabled: () =>
                            data.isLoading ||
                            data.templateForm.templateId.trim().length === 0,
                        onclick: publishTemplate,
                    },
                    t.span({ className: "txt" }, "Publish template"),
                ),
            ),
        );
    }

    function planSection() {
        return t.div(
            { className: "block m-b-base" },
            t.h5({ className: "txt-bold m-b-sm" }, "Draft a storage plan"),
            t.div(
                { className: "grid" },
                t.div(
                    { className: "col-lg-6" },
                    fieldEl(
                        "Layout",
                        app.components.select({
                            name: "layout",
                            options: ["shared", "user", "tenant", "agent"].map((l) => ({ value: l, label: l })),
                            value: () => data.planForm.layout,
                            onchange: (opts) => (data.planForm.layout = opts?.[0]?.value ?? data.planForm.layout),
                        }),
                        "Tenant layouts additionally need a tenant source and access composition — configure those through the API or SDK; this guided screen covers shared, user and agent layouts.",
                    ),
                ),
                t.div(
                    { className: "col-lg-6" },
                    fieldEl(
                        "Provision",
                        t.select({ disabled: true }, t.option({ value: "lazy", selected: true }, "lazy")),
                        "Databases are created on the first authorized access.",
                    ),
                ),
                t.div(
                    { className: "col-lg-6", hidden: () => !data.isPrivate },
                    fieldEl(
                        "Auth collections",
                        t.input({
                            value: () => data.planForm.authCollectionIds,
                            onchange: (e) => (data.planForm.authCollectionIds = e.target.value),
                            placeholder: "users, staff",
                        }),
                        "Comma separated auth collection ids or names that receive private databases.",
                    ),
                ),
                t.div(
                    { className: "col-lg-6", hidden: () => !data.isPrivate },
                    fieldEl(
                        "Template id",
                        t.input({
                            value: () => data.planForm.templateId,
                            onchange: (e) => (data.planForm.templateId = e.target.value),
                            placeholder: "tpl-user",
                        }),
                        "Required — a published template copied into every provisioned database. Publish one below if the list is empty.",
                    ),
                ),
                t.div(
                    { className: "col-lg-6", hidden: () => !data.isPrivate },
                    fieldEl(
                        "Namespace",
                        t.input({
                            value: () => data.planForm.namespace,
                            onchange: (e) => (data.planForm.namespace = e.target.value),
                        }),
                        "Prefix used for the provisioned database identifiers.",
                    ),
                ),
                t.div(
                    { className: "col-lg-6", hidden: () => !data.isPrivate },
                    fieldEl(
                        "Private collection",
                        t.input({
                            value: () => data.planForm.privateCollection,
                            onchange: (e) => (data.planForm.privateCollection = e.target.value),
                            placeholder: "notes",
                        }),
                        "The relocated collection; it must be empty.",
                    ),
                ),
            ),
            t.div(
                { className: "flex gap-10 m-t-base" },
                t.button(
                    {
                        type: "button",
                        className: () => `btn expanded ${data.isLoading ? "loading" : ""}`,
                        disabled: () => data.isLoading || data.applying || !data.hasState,
                        onclick: draftPlan,
                    },
                    t.span({ className: "txt" }, "Validate plan"),
                ),
                t.button(
                    {
                        type: "button",
                        className: "btn transparent secondary expanded",
                        hidden: () => !data.plan,
                        onclick: () => (data.plan = null),
                    },
                    t.span({ className: "txt" }, "Discard"),
                ),
            ),
        );
    }

    function planSectionResult() {
        if (!data.plan) return "";
        return t.div(
            { className: "alert info m-b-base" },
            t.div({ className: "txt-bold" }, "Validated plan"),
            t.div({ className: "m-t-xs txt-sm" }, () =>
                `Layout "${data.plan.layout}" — ${data.plan.plannedDatabases} planned database(s), state ${data.plan.state}, plan hash ${data.plan.planHash}.`,
            ),
            t.div(
                { className: "flex gap-10 m-t-sm" },
                t.button(
                    {
                        type: "button",
                        className: () => `btn danger expanded ${data.applying ? "loading" : ""}`,
                        disabled: () => data.applying || data.plan.state !== "ready",
                        onclick: applyPlan,
                    },
                    t.span({ className: "txt" }, "Apply plan"),
                ),
            ),
            t.div(
                { className: "txt-sm m-t-sm" },
                "Applying publishes the policy through the guarded durable path; a failed apply leaves the previous policy active.",
            ),
        );
    }

    function appliedNotice() {
        if (!data.applied) return "";
        return t.div(
            { className: "alert success m-b-base" },
            () =>
                `The plan is published. The active layout is now "${data.activeLayout}". Private databases provision lazily on first authorized access.`,
        );
    }

    return t.div(
        { pbEvent: "pageMirabaseSetup", className: "page page-mirabase-setup" },
        settingsSidebar(),
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, "Settings"),
                    t.div({ className: "breadcrumb-item" }, () => app.store.title),
                ),
            ),
            t.div(
                { className: "wrapper m-b-base" },
                t.div(
                    { className: "grid" },
                    t.div(
                        { className: "col-lg-12" },
                        t.div(
                            { className: "flex gap-10 m-b-sm" },
                            t.div({ className: "txt-lg" }, "MiraBase storage setup"),
                            app.components.refreshButton({
                                className: "btn sm transparent secondary circle tooltip-bottom",
                                onclick: loadState,
                            }),
                        ),
                        t.div(
                            { className: "txt-sm txt-hint m-b-base" },
                            "Choose how the application data is stored: the shared primary database, or a private database per user, tenant or agent. The preview explains the binding and the expected database count before anything is published; populated shared collections cannot be relocated in this release (layout_migration_required).",
                        ),
                        t.div(
                            { className: "alert danger m-b-base", hidden: () => !data.error },
                            () => data.error,
                        ),
                        () => appliedNotice(),
                        stateSection(),
                        templateSection(),
                        planSection(),
                        () => planSectionResult(),
                    ),
                ),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
