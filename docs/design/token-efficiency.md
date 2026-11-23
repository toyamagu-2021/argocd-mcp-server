# Token efficiency of tool results

- Status: **design only**. No tool behavior in this change.
- Issue: [#117](https://github.com/toyamagu-2021/argocd-mcp-server/issues/117)
- Reference: `get_application_managed_resources` as merged in #113 (`internal/tools/get_app_managed_resources.go` at `2da3566`)
- Base: `origin/main` `2da3566` (includes #113 and #114). 27 public tools, see `docs/todo-tools.md`.

Tool-result paging is an argument on the tool (`limit` / `offset`). It is not MCP `nextCursor`. The MCP pagination spec covers list methods, not tool results. The Argo CD RPCs these tools call do not take a page, so paging stays in the tool, over the slice the RPC already returned, which is what #113 does.

Helm values stay (D6 / D12). Secret redaction stays where Argo CD already does it. Hashing a field and returning the hash is still a non-goal.

## 1. Shape

Two shapes were sketched. The arena runners named by the architect workflow (`claude-fable-5-2-high`, `claude-opus-5-5-high-fast`, `gemini-4-neon-xhigh`, then the same families at a lower tier) were rejected for weekly usage, so there is no cross-judge and no graft. The pick below is the sketch.

### Chosen: helpers in `internal/tools/output.go`

Package `tools`. Unexported. Each tool keeps its own summary struct next to the handler and calls the helpers. `get_application_managed_resources` becomes a caller. Its JSON keys, notice sentences, default limit 50, and strip behavior do not change.

One owner for each of these:

| Concern | Owner |
|---|---|
| Page math and the `items` / `totalItems` / `hasMore` / `notice` object | `page` + `pageSlice` |
| Notice sentences | `pageNotice` (known total) and `tailNotice` (logs, total unknown) |
| Reading `limit` / `offset` from a tool call | `pageArgs` |
| Compact JSON bytes | `writeCompact` |
| `managedFields` and `kubectl.kubernetes.io/last-applied-configuration` | `stripObjectMeta` for `metav1.ObjectMeta`, `stripAnnotations` for a bare map, `stripManifestNoise` for a manifest string |

A tool author learns those functions. The tool still decides which fields are the summary, because that is the part that differs per API type.

### Rejected: a `View` interface in a new package

```go
type View interface {
    Summary() any
    Detailed() any
}
func Render(v View, detailed bool, limit, offset int) (*mcp.CallToolResult, error)
```

`Render` would page when the value is a slice, strip by JSON-walking the encoding, and be the only marshal path. That looks like one way in, and it is the wrong owner:

- Logs are text, and the server does not know the total line count. A page envelope would invent `totalItems`. A special case inside `Render` means `Render` knows every exception.
- `get_application` is one object. Paging it is meaningless. `Render` would have to tell objects from slices.
- Walking every JSON value through `map[string]any` randomizes nothing the struct encoder already orders, and it changes #113's output. #113 strips manifest strings only, then `json.Marshal`s the struct. PR U1 has to keep those bytes stable.
- `list_application` default output is TSV. A `View` has no place to return TSV without a side channel.
- Summary fields would still be written by the tool. The interface would be a wrapper with one caller.

D15 already says App JSON paths may be shared later and that a values-redaction render layer is unnecessary. `writeCompact` plus the strip helpers are that sharing. They are not a new render framework.

## 2. Helpers

File: `internal/tools/output.go`. Tests: `internal/tools/output_test.go`.

```go
const (
    lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
    defaultPageLimit      = 50
    defaultManifestLimit  = 10
    defaultTailLines      = 20
)

// page is the #113 response object. Notice is omitted when empty.
// Hosts is not a field of page. The resource tree embeds page and adds Hosts.
type page struct {
    Items      any    `json:"items"`
    TotalItems int    `json:"totalItems"`
    HasMore    bool   `json:"hasMore"`
    Notice     string `json:"notice,omitempty"`
}

// pageArgs reads limit and offset the way nonNegativeIntArg does today
// (missing -> def, JSON whole numbers accepted, fractions and negatives rejected).
// limit must be >= 1. There is no 0 = all. offset must be >= 0.
func pageArgs(request mcp.CallToolRequest, defaultLimit int) (limit, offset int, err error)

// pageSlice returns items[offset:offset+limit], clamped to the slice.
// It does not drop nils. Callers that need #113's nil drop filter first.
// The offset used for the slice is a local copy. Callers pass the original
// offset to pageNotice. Precondition: limit >= 1 and offset >= 0, which
// pageArgs already enforced.
func pageSlice[T any](items []T, limit, offset int) (sliced []T, total int, hasMore bool)

// pageNotice is the only sentence for a known total.
// filters is empty, or a noun list such as
// "kind, resource_name, resource_namespace, group, or version".
// The range is 0-based and inclusive: offset through offset+returned-1.
// "Showing items 1-2 of 3" is offset 1, returned 2, which is the #113 test string.
//
//	offset == 0 && !hasMore          -> ""
//	returned == 0 && filters == ""   -> "offset %d is past %d items."
//	returned == 0                    -> "offset %d is past %d items. Narrow with %s."
//	hasMore && filters == ""         -> "Showing items %d-%d of %d. Pass offset=%d to fetch more."
//	hasMore                          -> "Showing items %d-%d of %d. Pass offset=%d to fetch more, or narrow with %s."
//	otherwise                        -> "Showing items %d-%d of %d."
func pageNotice(offset, returned, total int, hasMore bool, filters string) string

// tailNotice is the log sentence. The RPC does not report a total.
// returned < requested -> "".
// otherwise -> "Showing the last %d lines (tail_lines=%d). The server does not report the total. Pass a larger tail_lines to see more."
func tailNotice(returned, requested int) string

func writeCompact(v any) (*mcp.CallToolResult, error) // json.Marshal, text result
                                                     // error text: "Failed to format response: %v"

func stripAnnotations(ann map[string]string) map[string]string // delete last-applied; nil if empty
func stripObjectMeta(meta *metav1.ObjectMeta)                  // ManagedFields = nil; Annotations = stripAnnotations

// stripManifestNoise is the #113 JSON object walk, moved unchanged in U1.
// U10 teaches it YAML: if the trim does not start with '{' or '[', decode with
// sigs.k8s.io/yaml (promote that module from indirect to direct in U10),
// delete the same two keys, and yaml.Marshal. Decode failure returns raw.
// YAML re-encoding can change key order and quoting. JSON behavior stays.
func stripManifestNoise(raw string) string
```

`managedResourcesPageArgs`, `nonNegativeIntArg`, `pageResourceDiffs`, `managedResourcesNotice`, and the private page struct in `get_app_managed_resources.go` are deleted in U1. The tool filters nil diffs, then calls `pageSlice`. Its `filters` argument is `kind, resource_name, resource_namespace, group, or version`, which keeps today's notice strings. `includeManagedResourceState`, `managedResourceSummary`, and `summarizeResourceDiffs` stay in that file.

Do not sort. Page order is the RPC order. Do not add a per-string length cap. A cap would be a second truncation mechanism, and hashing the cut text stays out of scope.

`defaultManifestLimit` is 10, not 50. A manifest document is a whole resource. Fifty of them recreate the problem #117 measures. List rows and event summaries use 50. The helper is the same `pageArgs`; only the default passed in changes.

`defaultTailLines` is 20. The measured payload at tail 100 is about 21.5KB of JSON. Twenty text lines is enough to see a crash loop, and `tail_lines` raises it with no hidden maximum. Logs do not grow an `offset`. The RPC is a tail.

## 3. Per tool

`detailed=false` is the default everywhere the flag exists. `detailed=true` does not disable paging. #113 already works that way. Empty `notice` is omitted. Compact JSON is one line from `writeCompact`.

Descriptions that gain `detailed`, `limit`, or `offset` say the default in the tool description, not only on the argument. Schema tests assert that sentence.

### `get_application`

Summary is one compact object, not a page. Reuse `ApplicationSummary` in `list_apps.go` so list and get do not grow two application summaries. Add these fields with `omitempty`. The list builder does not set them, so a list row does not gain keys until a value is present.

| Field | Source |
|---|---|
| existing `ApplicationSummary` fields | same mapping list uses today (`name`, `namespace`, `project`, `source`, `destination`, `syncStatus`, `healthStatus`, `operationStatus`) |
| `sources` | `spec.sources[]` as `ApplicationSourceBrief`, when `spec.sources` is set. `source` stays the single `spec.source`. Repo URL, path, target revision, chart only. No Helm values, parameters, or file parameters |
| `syncRevision` | `status.sync.revision` |
| `syncRevisions` | `status.sync.revisions`, when non-empty |
| `healthMessage` | `status.health.message` |
| `historyCount` | `len(status.history)` |
| `healthCounts` | count of `status.resources` by `health.status` |
| `syncCounts` | count of `status.resources` by `status` (the sync code) |
| `conditions` | `status.conditions[]` as `{type, message}` |

`status.resources` entries, history bodies, and `operationState.syncResult` are not in the summary. The description points at `get_application_resource_tree` and `get_application_managed_resources` for per-resource data.

`detailed=true`: the full `Application` after `stripObjectMeta`, compact. Helm values stay. This can still be large. That is the opt-in.

### `get_application_resource_tree`

Combine `nodes` then `orphanedNodes` into one slice. `totalItems` counts that slice. Order is the RPC order. Each summary item:

| Field | Source |
|---|---|
| `group`, `version`, `kind`, `namespace`, `name`, `uid` | `ResourceRef` |
| `health` | `health.status` only |
| `parentRefs` | `group`, `kind`, `namespace`, `name` (no uid) |
| `orphaned` | true for `orphanedNodes`, omitted otherwise |

Omit `info`, `networkingInfo`, `images`, `resourceVersion`, `createdAt`.

```go
type resourceTreeView struct {
    page
    Hosts any `json:"hosts,omitempty"`
}
```

Summary `hosts` is the host `name` strings. Detailed `hosts` is the full `[]HostInfo`, including `systemInfo`, and it is repeated on every page so paging nodes does not drop the cluster. `NodeSystemInfo` is a known part of the 26,451-byte fixture. The default summary drops it. Detailed is allowed to stay large; the description says so.

`detailed=true` pages `resourceNodeDetailed`:

```go
type resourceNodeDetailed struct {
    v1alpha1.ResourceNode `json:",inline"`
    Orphaned              bool `json:"orphaned,omitempty"`
}
```

Nodes have no `managedFields`. No strip on the node. `filters` is empty. The notice only tells the next `offset`.

### `get_application_events`

Page of summaries. `filters` is `resource_name, resource_namespace, or resource_uid`. Existing resource filters stay and apply before paging (they are RPC filters).

| Field | Source |
|---|---|
| `type`, `reason`, `message`, `count` | the event |
| `firstTimestamp`, `lastTimestamp` | RFC3339 from the proto timestamps, omitted when zero |
| `involvedKind`, `involvedNamespace`, `involvedName` | `involvedObject` |
| `sourceComponent` | `source.component` |

No `metadata` on the summary. That is where `managedFields` sits, and it is the bulk past the message. The message is not clipped.

`detailed=true`: page of `eventspkg.Event` after `stripObjectMeta(&event.Metadata)` on each item, compact. Same `limit` / `offset`.

### `get_application_logs`

No `limit` / `offset`. Default `tail_lines` changes from 100 to 20. `follow=true` stays an error (D18).

Default body is text, not JSON:

```
application=<name> pod=<pod> container=<container> lines=<n>
<timestamp>\t<pod>\t<content>
```

Empty timestamp or pod keeps the tabs. Content is already trimmed of a trailing newline. When `tailNotice` is non-empty, it is the last line.

`detailed=true`: today's `LogResponse` (`application`, `pod_name`, `container`, `total_lines`, `logs[]`) as compact JSON, plus `notice` when `tailNotice` is non-empty. `detailed` does not raise the tail. `total_lines` is the number returned, not the size of the pod log.

### `list_application`

Keep `output_format=tsv` as the default. The description already says TSV is the default. Keep `name_only` winning over `detailed`. Keep the summary columns and `optional_fields`.

Paging happens after the existing project / cluster / namespace / selector filtering, before rendering. Default `limit` 50, `offset` 0.

| Call | Body |
|---|---|
| `output_format=tsv`, not `detailed`, not `name_only` | today's summary TSV of the page. If `pageNotice` is non-empty, a final line `# ` plus that sentence |
| `output_format=tsv`, `name_only=true` | today's newline-separated names of the page, then the same `# ` line when the notice is non-empty |
| `detailed=true` (any `output_format`) | page envelope. `items` are full Applications after `stripObjectMeta`. Compact |
| `output_format=json`, `name_only=true` | page envelope. `items` are name strings. `totalItems` replaces the old `count` |
| `output_format=json` otherwise | page envelope of `ApplicationSummary` |

`filters` is `project, cluster, namespace, app_namespace, or selector`.

Today `detailed=true` with the default TSV format calls `generateDetailedTSV`, which returns a compact JSON array, not TSV. That path becomes the page envelope. One JSON shape for detailed, instead of a raw array that only happens when the format is TSV.

Empty input is no longer the prose `No applications found matching the criteria.`

| Format | Empty body |
|---|---|
| summary TSV | the header row only |
| name-only TSV | empty text |
| JSON | `{"items":[],"totalItems":0,"hasMore":false}` |

`#` is safe as a trailer prefix. Kubernetes object names cannot contain `#`.

### `list_cluster` and `list_repository`

Both gain `limit` / `offset` / the page envelope. `filters` is empty.

`list_cluster` already has summary and `name_only`. Summary items stay `ClusterSummary`. `name_only` items stay `{name, server}` objects inside `items` (not a `{clusters, count}` wrapper). `detailed=true` pages `clusters.Items` after `Sanitized()` and `stripAnnotations` on `Annotations`. Today detailed marshals the whole list object. The page replaces that wrapper.

`list_repository` gains a summary, which it does not have today. Summary item: `repo`, `type`, `name`, `project`, `connectionStatus` (`connectionState.status`). `detailed=true` pages the sanitized repository. There is no annotation field to strip. No `name_only` on this tool. Do not add one.

### `list_project` and `list_applicationset`

Page envelope, default limit 50. Empty prose goes away, same JSON empty page as the other lists.

`list_project` summary item: `name`, `description` (`spec.description`), `sourceRepoCount`, `destinationCount`, `roleCount`. Roles, JWT tokens, sync windows, and allow-lists are detailed-only. `name_only` stays and wins: `items` are name strings. `detailed=true` is not a flag today. Add it, default false, paging full projects after `stripObjectMeta`. `filters` is empty.

`list_applicationset` summary item: `name`, `namespace`, `project` (`spec.template.spec.project`), `generatorCount`. `detailed=true` is new, default false, paging the full ApplicationSet after `stripObjectMeta`. Generators and the template, including Helm values inside the template, are detailed-only. `filters` is `project or selector`. Existing filters still run before paging.

### `get_project` and `get_applicationset`

One object, not a page. Summary fields match the list item for that type. `detailed=true` is the full object after `stripObjectMeta`, compact. Helm values inside an ApplicationSet template appear only when `detailed=true`.

### `get_application_manifests`

The point of this tool is the manifest text. The resource tree already summarizes resources, so this tool does not grow a summary that drops the YAML. It pages `manifests`. Default `limit` is `defaultManifestLimit` (10). Each string goes through `stripManifestNoise`. The page `items` are the stripped strings. Sibling fields from `ManifestResponse` are copied onto a small struct that embeds `page`: `namespace`, `server`, `revision`, `sourceType`. `commands` and the integrity fields ride along only when `detailed=true`, because they are not what an agent needs to read a page of manifests. `filters` is empty. `revision` stays the git revision argument, not a page control.

### `get_application_resource`

One manifest. No page and no summary. `stripManifestNoise` on `manifest`, then `writeCompact` of the response. A nil manifest stays omitted (`omitempty`).

### Tools that stay a full object

Compact, strip where the type has the field, no summary default. The caller just asked for this object, or just performed the write and needs the result (for sync, that includes `operationState`). A summary default here would hide the result the tool exists to return. They can still be large. The description says to call `get_application` when only the summary is needed.

| Tool | Strip | Body |
|---|---|---|
| `create_application`, `sync_application`, `refresh_application`, `rollback_application` | `stripObjectMeta` | full `Application` |
| `update_application` | none (the body is `ApplicationSpec`) | full spec, Helm values included |
| `create_project`, `create_applicationset` | `stripObjectMeta` | full object |
| `get_cluster` | `stripAnnotations` on `Annotations` | full sanitized cluster |
| `get_repository` | none | full sanitized repository |
| `get_user_info` | none | full object |

`refresh_application` returns the app before reconcile finishes. Leaving the full object in place avoids a summary that looks like a finished refresh.

Delete and terminate responses are short text. They stay as they are.

## 4. Implementation units

Ship user-visible units in **0.4.0**. 0.3.0 is already tagged, and several defaults below change what an existing client parses. U1 has no behavior change and can merge first. Do the units in order. Each one is a separate PR. Do not stack them into one branch.

Every behavior unit updates that tool's description and the notes cell in `docs/todo-tools.md` in the same PR. Schema tests assert the new default is in the description.

Size measurement is a Go test, not a live cluster. Log `before_bytes` and `after_bytes`. `before` is `json.MarshalIndent` of the fixture the way the handler marshals today. `after` is the new body. Copy the log line into the PR. The fixture is synthetic and built in the test, following `sampleManagedResources`. Assert a ratio so a later edit cannot silently undo the cut. Do not invent the byte counts in advance.

| Unit | Change | Tests | Size |
|---|---|---|---|
| U1 | Move helpers into `output.go`. Managed resources calls them. No request or response change | `output_test.go` table tests for `pageArgs`, `pageSlice`, `pageNotice` (including the existing `Showing items 1-2 of 3` string and the managed-resources narrow sentence), `stripManifestNoise`. Existing `get_app_managed_resources_test.go` stays green | `TestManagedResourcesSummarySize` logs the same three counters. PR says the numbers did not move |
| U2 | `get_application` summary / detailed | Handler test: summary lacks `spec`, `status.resources`, `managedFields`, Helm value text. `detailed=true` contains `spec` and a fixture value, and lacks `managedFields` and the last-applied annotation. Description mentions `detailed=false` | One Application fixture with managedFields, a 2KB values string, 30 `status.resources`, 10 history entries. Assert summary under 1/5 of indented full |
| U3 | Resource tree summary page | Summary has no `systemInfo`, `networkingInfo`, or `images`. `orphaned` set only on the second group. `limit=1` sets `hasMore` and the notice's next offset. Detailed includes `networkingInfo` | 40 nodes, one with networking and images, two hosts with a filled `NodeSystemInfo`. Assert summary page under 1/5 of indented full |
| U4 | Events summary page | Summary has no `metadata`. Detailed has no `managedFields`. Past-the-end offset returns the notice and `items: []` | 40 events with managedFields and a 400-byte message. Assert summary page under 1/5 of indented `EventList` |
| U5 | Logs text, tail 20 | Default call uses tail 20. Text body has no `{`. `detailed=true` is compact JSON with `logs`. Omitted `tail_lines` is 20, not 100. `follow=true` still errors. Notice present when returned == tail, absent when the stream is shorter | 100 lines of 80 bytes. Assert default text under 1/5 of indented JSON at tail 100 |
| U6 | `list_application` paging, empty envelope, detailed page | TSV header unchanged for a short list and `offset=0`. 60 apps, default limit, TSV row count 50 plus a `# Showing` line. JSON has `totalItems` and `hasMore`. `name_only` wins. Empty list is the empty page or the header, not the old prose. Update `list_apps_test.go` | 60 applications with values set. Default summary TSV under 1/5 of indented full JSON. Detailed page under indented full |
| U7 | `list_cluster`, `list_repository` | Cluster summary keys unchanged, wrapped in `items`. Repository summary has no `username` or `password`. Empty lists are empty pages. Update `list_cluster_test.go`, `list_repository_test.go`, and the empty-prose checks in `test/argocd_e2e/repository_test.go` | 40 clusters and 40 repositories. Summary page under 1/5 of indented full |
| U8 | `list_project`, `list_applicationset` | Project summary has no `roles`. ApplicationSet summary has no `generators` and no Helm values. `detailed=true` has them and no `managedFields`. Update `list_projects_test.go`, `list_applicationset_test.go`, `test/argocd_e2e/project_test.go`, `test/mock_argocd_e2e/applicationset_e2e_test.go` | 20 projects with a role JWT-sized string, 20 ApplicationSets with a values string in the template. Summary page under 1/5 of indented full |
| U9 | `get_project`, `get_applicationset` | Same field rules as the list item, one object, no `totalItems`. Detailed is the stripped object | One fat project and one fat ApplicationSet. Summary under 1/5 of indented full |
| U10 | Manifest paging, YAML strip, `get_application_resource` | 15 manifests, default page length 10, `hasMore` true, notice offset 10. JSON manifest loses managedFields. YAML manifest loses the annotation and is still YAML (does not start with `{`). Single-resource manifest is stripped and compact. `sigs.k8s.io/yaml` becomes a direct require | 15 JSON manifests of ~2KB with managedFields. Default page under 1/2 of indented `ManifestResponse`. One YAML fixture asserts the annotation is gone |
| U11 | Full-object compact + strip for the write tools, `get_cluster`, `get_repository`, `get_user_info` | Each handler's success body is one line, valid JSON, and a fixture Application has no `managedFields`. Spec from `update_application` still contains values. Cluster annotations lose the last-applied key. User info still contains the subject | One Application through the sync handler, same shape as U2. Compact stripped body smaller than indented full, and smaller than indented stripped (indent is part of the win) |

U2 also edits `docs/design/refactor-0.3.md` only if this design's section 7 wording is not already on main. It is applied with this design, so U2 must not revert it.

Handler tests call the `xHandler` function with a mock client, the same way current tool tests do. No Kind cluster for the size number.

## 5. Compatibility

These are breaking changes for any client that parses tool text. The server has no output schema version. 0.4.0 is the release, and the changelog lists every row below. Tool descriptions are the only in-band warning an agent will see.

| Tool | What changes for a client that omits the new arguments |
|---|---|
| `get_application` | Indented full Application becomes a one-line summary. `spec` and `status.resources` disappear unless `detailed=true` |
| Resource tree | Indented tree becomes a page of node summaries. `systemInfo` disappears unless `detailed=true`. `hosts` on the summary are names |
| Events | Indented `EventList` (`metadata`, `items`) becomes a page of summaries. `metadata` disappears unless `detailed=true` |
| Logs | JSON object becomes tab text. Default tail is 20, not 100. `detailed=true` restores JSON, still at tail 20 unless `tail_lines` is set |
| `list_application` | At most 50 rows unless `limit` is set. A `# ` notice line appears on TSV when the page is short of the total. JSON list responses become a page object. `name_only` JSON loses the `{names, count}` wrapper. `detailed` plus default TSV stops being a raw JSON array. The empty prose string goes away |
| `list_cluster` | Raw summary array (or `{clusters, count}`, or the list wrapper when detailed) becomes a page. At most 50 items |
| `list_project`, `list_applicationset`, `list_repository` | Full objects become summary pages. Empty prose goes away |
| `get_project`, `get_applicationset` | Full object becomes a summary |
| Manifests | All documents become the first 10, inside a page. YAML text may be re-quoted |
| `get_application_resource` and the U11 tools | Indentation becomes compact. `managedFields` and the last-applied annotation disappear. Keys that remain are the same |

Clients that only read TSV columns by header name keep working for lists of 50 or fewer, with no `#` line. Clients that split TSV on every newline must ignore a trailing `#` line.

`ARGOCD_MCP_READ_ONLY` does not change. No tool is added or removed. `publicTools` in `register_test.go` stays. Argument names that exist today keep their meaning. `limit` and `offset` are new and optional.

### Risks

- An agent that used `get_application` as the resource list now has to call the tree or managed resources. The summary's `healthCounts` and `syncCounts` are there so it can still see that something is degraded without the resource list.
- Detailed tree pages repeat `HostInfo.systemInfo`. A cluster with many nodes can make `detailed=true` large on every page. The default avoids it. If a later measurement shows the detailed page is still over the client's cap, drop `systemInfo` from detailed hosts in a follow-up. Do not do that inside these units without a new size number.
- `stripManifestNoise` on YAML re-encodes the document. A caller diffing manifest text against git will see formatting noise. JSON manifests already re-encode today on the managed-resources path.
- Paging is not a cursor. An insert or delete between calls can shift a later page. The notice does not claim otherwise.
- `pageSlice` does not drop nils. Forgetting the nil filter in managed resources would change `totalItems`. U1's existing test covers a non-nil fixture; add one nil item to `output`/`managed resources` test so the filter stays put.
- `writeCompact` on a value that still contains `managedFields` is a bug. U11's test is the guard for write tools. Each detailed path's test is the guard for that tool.
- E2E and mock tests that accept the empty-list prose have to learn the empty page in the same unit as the tool. The files are listed on U6, U7, and U8.

## 6. Non-goal in `docs/design/refactor-0.3.md`

Section 7 used to end with 「長さ制限・ハッシュ」. Length limits of tool results are now this document. Hashing a field value is still a non-goal. The section 7 sentence is updated in this design change, and implementation units must leave it in place. D1, D6, and D12 are unchanged. D15's shared App JSON path is `writeCompact` and the strip helpers, not a render package.
