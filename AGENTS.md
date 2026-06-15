# AGENTS.md

Context for coding agents working on `aws-terminal`.

## Project purpose

`aws-terminal` is a Go terminal UI for creating and managing AWS resources. It uses Bubble Tea for the shell/TUI and AWS SDK for Go v2 for AWS integrations.

Current implemented workflows:

- AWS profile discovery from shared config/credentials.
- Native AWS SSO OIDC device-flow login without requiring the AWS CLI for login.
- Region selection and active session/caller identity resolution.
- S3 bucket listing and local file/folder sync to S3.
- CloudFront distribution listing and invalidation creation/polling.
- ECR private repository listing/creation and local Docker image push workflows.
- ECS cluster, service, and running-task browsing with compact service/task detail views.
- Implemented pages are currently Dashboard, S3, CloudFront, ECR, and ECS. IAM, EC2, and Networking placeholders have been removed until real workflows are added.

## Common commands

```bash
go run ./cmd/aws-terminal
go test ./...
go build ./...
gofmt -w <changed-go-files>
```

Manual AWS verification depends on locally configured AWS profiles and real AWS resources.

## Architecture map

```text
cmd/aws-terminal                         # app entrypoint
internal/app                             # dependency wiring, app page registry, and Bubble Tea program bootstrap
internal/domain/*                        # core types only; no UI or AWS SDK imports
internal/application/*                   # use cases + small ports/interfaces
internal/infrastructure/*                # AWS SDK and filesystem/shared-config adapters
internal/ui/pageapi                      # shared Page contract, State, OpenPageMsg, OwnedMsg, page Status
internal/ui/workflow                     # reusable UI workflow helpers/status/session-key helpers
internal/ui/shell                        # main Bubble Tea model, focus, key handling, layout
internal/ui/components                   # header/footer/sidebar components
internal/ui/pages                        # page type re-exports and simple pages
internal/ui/pages/s3                     # S3 workflow package
internal/ui/pages/cloudfront             # CloudFront workflow package
internal/ui/pages/ecr                    # ECR workflow package
internal/ui/pages/ecs                    # ECS browser/detail package
internal/ui/styles                       # shared Lip Gloss theme/helpers
```

Dependency direction should stay:

```text
ui -> application -> domain
infrastructure -> application/domain
app wires ui + infrastructure together
```

Keep AWS SDK imports out of `domain`, `application`, and `ui` packages. Add small ports in `internal/application/.../ports.go` and implement them in `internal/infrastructure/...`.

## App wiring

`internal/app/app.go` constructs:

- `session.Service` with shared-config profile repository + STS identity resolver.
- `authentication.Service` with native SSO OIDC device-flow authenticator.
- `s3.Service` with AWS S3 store.
- `cloudfront.Service` with AWS CloudFront service.
- `ecr.Service` with AWS ECR service + local Docker adapter.
- `ecs.Service` with AWS ECS service.
- app page registry via `internal/app/pages.go`.
- `shell.NewModelWithPreferences(...)`, run with `tea.WithAltScreen()`.

## Shell/TUI mental model

Main shell files:

- `internal/ui/shell/model.go`: top-level state and helpers.
- `internal/ui/shell/update.go`: Bubble Tea update loop, focus switching, profile activation.
- `internal/ui/shell/view.go`: high-level layout and footer status composition.
- `internal/ui/shell/*_list.go`: sidebar list construction/selection.
- `internal/ui/shell/keymap.go`: global key bindings.

Focus areas cycle with `tab`:

1. Profiles
2. Regions
3. Pages
4. Active page workflow only after pressing `enter` on Pages

`shift+tab` moves backward. Pages receive workflow key messages only when `State.PageFocused` is true.

Global keys:

- `q` / `ctrl+c`: quit
- `tab`: next focus
- `shift+tab` / `backtab`: previous focus
- `enter`: apply/open for focused sidebar pane
- `r`: refresh profiles

Async page messages implement `pageapi.OwnedMsg` and are routed by the shell to their owning page even if that page is not currently visible.

## Page contract

The shared page contract lives in `internal/ui/pageapi`. `internal/ui/pages/page.go` re-exports these types for compatibility.

```go
type Page interface {
    ID() string
    Title() string
    Description() string
    OnStateChanged(state State) tea.Cmd
    SetFocused(focused bool) tea.Cmd
    Update(msg tea.Msg, state State) tea.Cmd
    View(state State, width, height int) string
    ShortHelp() []key.Binding
    FullHelp() [][]key.Binding
}
```

Optional page status:

```go
type StatusProvider interface {
    PageStatus(state State) Status
}
```

Important patterns:

- `OnStateChanged` is called when shell state changes (profile/session/region/focus/status).
- Pages should reset session-specific state when profile/region session keys change.
- Pages should ignore workflow key handling unless `state.PageFocused` is true.
- Use page-specific typed `tea.Msg` values for async commands.
- Async messages that may arrive after navigation should implement `OwnerPageID() string`.
- If a page owns text inputs, focus/blur them in `SetFocused` based on current stage.
- Register new pages in `internal/app/pages.go`; keep `internal/ui/shell` page-agnostic.
- Use `internal/ui/workflow.SessionKey`, `ActiveRegion`, and status helpers for staged AWS workflow pages.

## Page package organization

Large workflows are grouped into subpackages to avoid very large files:

```text
internal/ui/pages/s3/
  s3.go             # types, constructor, static Page metadata
  s3_update.go      # state transitions and key handling
  s3_view.go        # rendering/help
  s3_commands.go    # tea.Cmd creators and cancellation
  s3_helpers.go     # local helpers delegating shared workflow helpers where possible
  s3_keys.go        # key bindings
  s3_status.go      # optional PageStatus implementation

internal/ui/pages/cloudfront/
  cloudfront.go
  cloudfront_update.go
  cloudfront_view.go
  cloudfront_commands.go
  cloudfront_helpers.go
  cloudfront_keys.go
  cloudfront_status.go
```

This required the neutral `internal/ui/pageapi` package to avoid parent/child package import cycles.

## Adding a new AWS resource workflow

Recommended steps:

1. Add domain types in `internal/domain/<resource>/types.go`.
2. Add application port/use case in `internal/application/<resource>/`.
3. Add AWS SDK adapter in `internal/infrastructure/aws<resource>/`.
4. Add page package under `internal/ui/pages/<resource>/` implementing `pageapi.Page`.
5. Use `internal/ui/workflow` for active region, session key, fallback values, and page status helpers.
6. Add owned async messages where needed.
7. Wire the service in `internal/app/app.go` and register the page in `internal/app/pages.go`.
8. Add unit tests at the application layer using fake ports.
9. Add UI state-transition tests for the page package.

Prefer staged workflows: select resource -> enter/edit options -> review -> execute -> result.

## S3 workflow notes

Key files:

- `internal/application/s3/service.go`
- `internal/application/s3/ports.go`
- `internal/infrastructure/awss3/store.go`
- `internal/ui/pages/s3/*`
- `internal/domain/s3/types.go`

Current behavior:

- Lists buckets for active profile/region.
- File picker accepts both files and directories.
- Directory sync keys are relative to the selected directory, like `aws s3 sync <dir> s3://bucket/<prefix>`.
- Prefixes are normalized with no leading/trailing slash.
- Uploads are planned for all local files to refresh content and metadata; same-size remote objects are not skipped.
- Delete is explicit, never implicit, and disabled for single-file sources.
- Delete planning is scoped to objects under the selected prefix.
- S3 review uses a viewport for long upload/delete/skip plans.
- `esc` cancels/backs out of page workflow stages; running sync cancellation uses context cancellation.
- `awss3.contentTypeForPath` uses `mime.TypeByExtension` plus frontend-oriented fallbacks (`.webmanifest`, `.map`, fonts, images, etc.).
- Upload execution uses up to 8 workers; delete happens after uploads.

When modifying S3 sync behavior, run and/or update:

```bash
go test ./internal/application/s3 ./internal/infrastructure/awss3 ./internal/ui/pages/s3
go test ./...
```

## CloudFront workflow notes

Key files:

- `internal/application/cloudfront/service.go`
- `internal/application/cloudfront/ports.go`
- `internal/infrastructure/awscloudfront/service.go`
- `internal/ui/pages/cloudfront/*`
- `internal/domain/cloudfront/types.go`

Current behavior:

- Lists distributions.
- Accepts one or more invalidation paths from a text input.
- Normalizes paths to start with `/` and de-duplicates them.
- Creates invalidations and polls until status is `Completed`.
- `esc` stops waiting for invalidation status; already-created invalidations may still continue in CloudFront.
- Can copy an equivalent AWS CLI invalidation command to the clipboard.

## ECS workflow notes

Key files:

- `internal/application/ecs/service.go`
- `internal/application/ecs/ports.go`
- `internal/infrastructure/awsecs/service.go`
- `internal/ui/pages/ecs/*`
- `internal/domain/ecs/types.go`

Current behavior:

- Lists ECS clusters for the active profile/region.
- Selecting a cluster loads services and non-stopped tasks.
- Services and tasks are searchable and paginated in table views.
- Service detail uses a compact overview: status/counts first, optional `Needs attention`, then network, runtime, deployments, and identifiers.
- Task detail uses a compact overview: status/health first, optional `Needs attention`, then private IP, availability zone, runtime/connectivity, containers, network attachments, and identifiers.
- Task container image references are shortened in the overview to keep full ECR URLs from overwhelming the screen.
- Long ARNs remain visible but are de-emphasized in the bottom `Identifiers` section.
- `taskDetailLines()` delegates to `taskOverviewLines()` so a future task logs tab can be added without reshaping the overview renderer.
- Future task-detail tabs should use left/right or `[`/`]`; keep `tab` reserved for shell focus movement.

When modifying ECS behavior, run and/or update:

```bash
go test ./internal/application/ecs ./internal/infrastructure/awsecs ./internal/ui/pages/ecs
go test ./...
```

## Cancellation and async commands

- Long-running shell/page commands use cancellable contexts where practical.
- Page/session resets cancel stale page commands.
- Quitting cancels shell profile/auth commands.
- `context.Canceled` should generally be ignored or rendered as friendly cancellation status, not as a scary failure.
- Bubble Tea commands should return typed messages, not mutate page/shell state directly from goroutines.

## Authentication/session notes

Key files:

- `internal/application/session/service.go`
- `internal/application/authentication/service.go`
- `internal/infrastructure/awsconfig/*.go`
- `internal/infrastructure/awssso/device_flow.go`

Profile loading:

- Reads `~/.aws/config` and `~/.aws/credentials` unless overridden by `AWS_CONFIG_FILE` or `AWS_SHARED_CREDENTIALS_FILE`.
- Supports `[profile name]`, `[default]`, and `[sso-session name]` sections.
- Sorts `default` first, then profile names alphabetically.

Activation:

- Non-SSO profiles resolve caller identity through STS.
- SSO profiles start native OIDC device flow, poll, write token cache, then resolve identity.
- Region preference considers active region/session, env vars, profile default region, then `eu-west-1`.

## Testing guidance

- Use application-layer fake ports for most logic tests.
- Avoid live AWS calls in unit tests.
- Keep domain/application tests deterministic and filesystem-isolated with temp dirs.
- For Bubble Tea pages, test helpers and state transitions in the page subpackage.
- Use fake services for page tests; do not hit live AWS.
- Always run `gofmt` on changed Go files.
- Run `go test ./...` and `go build ./...` before handing off if dependencies/toolchain are available.

## Style/conventions

- Keep domain types simple and AWS-SDK-free.
- Trim/validate user-facing inputs in application services.
- Sort returned lists/plans for stable display and tests.
- Use explicit confirmation/review screens before destructive actions.
- Keep delete/destructive behavior opt-in.
- Use `context.Context` in application ports that may call external services.
- Keep `View()` pure and cheap; no I/O or AWS calls from render paths.
- Prefer centralized key bindings and `key.Matches` over raw `msg.String()` checks.
- Prefer viewport components for long/scrollable content.
