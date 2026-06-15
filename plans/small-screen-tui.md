# Small-screen TUI readability plan

## Context
- The TUI is difficult to read on a 14-inch MacBook-sized terminal.
- Pain points from the screenshot: ECS logs wrap awkwardly, long task/container names consume too much horizontal space, and keybind help takes significant vertical/horizontal space.
- Initial code scan points to a Go Bubble Tea/Lip Gloss app with reusable shell/page components under `internal/ui`.

## Approach
- Improve responsive layout behavior rather than changing core navigation.
- Add compact breakpoints for medium-height/medium-width terminals so the main page gets more usable space.
- Collapse the sidebar automatically on smaller screens when focus is in the Page area, giving the active page the full content width. Keep sidebar visible when focus is on Profiles, Regions, or Pages so navigation remains discoverable.
- Apply compact treatment across all pages, not only ECS, by improving the shared shell/footer behavior and pruning verbose page-local instructions.
- Keep compact keybind help as a useful one-line summary of all relevant keys for the current focus/stage rather than hiding it behind a separate help screen.
- For ECS tasks, preserve visibility of the full task definition because task IDs are not descriptive enough. Do not solve task table pressure by truncating away the task definition; instead prioritize task definition width, collapse sidebar, and reduce less useful columns/metadata when constrained.
- Prefer truncation and progressive disclosure for secondary metadata/help text over adding new panes or modes.

## Files to modify
- `internal/ui/shell/view.go` — header/footer/sidebar composition, auto-collapsed sidebar when `focusPage` is active on constrained screens, and condensed help/status behavior.
- `internal/ui/shell/helpers.go` — sidebar width/height, pane allocation breakpoints, and a helper for when the sidebar should collapse.
- `internal/ui/components/footer.go` — footer line budget, wrapping/truncation, and compact rendering.
- `internal/ui/components/sidebar.go` — optional compact sidebar hint/section spacing behavior.
- `internal/ui/components/header.go` — already has width-based status hiding; may need medium-width truncation.
- `internal/ui/pages/ecs/ecs_view.go` and `internal/ui/pages/ecs/ecs_helpers.go` — ECS task table priority for task definition, task log header, viewport sizing, log wrapping, and page-local help text.
- `internal/ui/pages/s3/s3_view.go` — shorten verbose inline stage instructions and ensure review/source content benefits from collapsed sidebar width.
- `internal/ui/pages/ecr/ecr_view.go` — shorten verbose inline workflow/search instructions and use shared compact footer behavior.
- `internal/ui/pages/cloudfront/cloudfront_view.go` — shorten verbose inline stage instructions and use shared compact footer behavior.
- `internal/ui/pages/ecs/ecs_view_test.go`, `internal/ui/shell/*_test.go`, and page-specific view tests where needed — regression coverage for compact layouts.

## Reuse
- Existing Bubble Tea viewport in `internal/ui/pages/ecs/ecs.go` and render helpers in `internal/ui/pages/ecs/ecs_view.go`.
- Existing page help contracts: `ShortHelp()` / `FullHelp()` across pages, which can be rendered into the new one-line compact footer help for the current stage.
- Existing focus model in `internal/ui/shell/model.go` / `internal/ui/shell/update.go`: `focusPage` already tracks when the user is interacting with the page, so sidebar collapse can be derived without adding a new persisted state.
- Existing breakpoints in `internal/ui/shell/helpers.go`: stacked sidebar below width 72, sidebar width clamped to 28–38, sidebar hint shown only when content height is at least 18.
- Existing footer fallback in `internal/ui/shell/view.go`: below width 64 it switches to `condensedHelpText()`.
- Existing header fallback in `internal/ui/components/header.go`: below width 52 it shows only the title.
- Existing `compactStatusText()` in `internal/ui/shell/view.go` for status truncation.

## Steps
- [ ] Inspect shell layout, footer/help rendering, sidebar sizing, and ECS log rendering.
- [ ] Define compact behavior for the screenshot-sized terminal class (about 150×43) and smaller terminals.
- [ ] Add an auto-collapse rule for the sidebar when `focusPage` is active on constrained screens, with a clear footer/header hint that `tab`/`shift+tab` brings navigation back.
- [ ] Update shared footer so keybind help renders as a single compact line at medium widths, not only below 64 columns, while still listing the useful current-stage bindings.
- [ ] Trim or prioritize footer status parts on constrained heights/widths so content loses less vertical space.
- [ ] Tune sidebar dimensions and hint visibility so the sidebar does not dominate at MacBook-sized widths/heights.
- [ ] Replace verbose inline key instructions across ECS, S3, ECR, and CloudFront with short prompts that defer detailed keys to the compact footer.
- [ ] Update ECS task list/table behavior to prioritize full task definition visibility: collapse sidebar while page-focused, allocate more width to the task definition column, and consider hiding/reducing secondary columns such as task ID/time/IP first on constrained widths.
- [ ] Update ECS logs view to preserve full task definition context where it identifies the task, increase viewport height where possible, and replace verbose inline instructions with compact help.
- [ ] Improve log wrapping so timestamps/prefixes consume less visible width on narrow log panes while retaining severity highlighting.
- [ ] Add or update tests for responsive footer/sidebar, ECS task definition visibility, and representative page compact rendering behavior.

## Verification
- [ ] Run `go test ./internal/ui/...`.
- [ ] Manually run the TUI at representative terminal sizes such as 150×43, 120×35, and <72 columns.
- [ ] In ECS task list/details/logs with page focus active, verify the sidebar collapses, full task definitions remain visible enough to identify tasks, log messages are readable, and one-line keybind/status text does not crowd the screen.
- [ ] Spot-check S3, ECR, and CloudFront at the same sizes to confirm compact footer/help behavior and page-local instruction cleanup work consistently.
