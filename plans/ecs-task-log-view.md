# ECS Task Log View Plan

## Context
- Goal: add a log view inside the existing ECS task browsing workflow so users can inspect task/container logs without leaving `aws-terminal`.
- Scope for this first pass is currently visible non-stopped tasks only; the existing application service filters out stopped tasks, and adding stopped-task browsing can be handled as a later feature.
- Current ECS page supports clusters -> services/tasks -> service/task details, but task detail only shows metadata, network, containers, and identifiers.
- Current ECS domain/application/infrastructure code only wraps ECS/EC2 APIs; there is no CloudWatch Logs client or log domain model yet.

## Approach
- Extend the existing ECS feature vertically: domain types, application API/service, AWS infrastructure, and ECS Bubble Tea page.
- Support only the standard ECS `awslogs` CloudWatch Logs driver for the first version; show a friendly empty state, not a failure, for other log drivers or missing log config (including the selected container name).
- Resolve a task/container's CloudWatch log configuration from the task definition, then fetch recent CloudWatch Logs events for one selected container at a time. Split the application/API surface into `DescribeTaskLogTargets` for resolving/caching container log targets and `FetchTaskLogEvents` for repeated event polling. Cache resolved task-definition log config in ECS page/session state for the selected task definition to avoid repeated `DescribeTaskDefinition` calls while switching containers or re-entering logs. Initial history should be bounded to the last 15 minutes and capped at 500 events to protect TUI responsiveness.
- Convert task detail into a tabbed view with `Overview` and `Logs` tabs. Keep the existing task overview content under `Overview`; automatically load CloudWatch logs when `Logs` is opened, then poll/stream for newer events every 3 seconds only while the user is actively viewing the ECS task `Logs` tab. Always stop polling when the user leaves the logs tab/task detail/ECS page focus; restart only by opening the logs tab again. If CloudWatch Logs returns an AWS error, stop streaming and show the error until the user changes container or re-enters the Logs tab.
- Render logs in a `bubbles/viewport` so the user can scroll through multiline output without overflowing the ECS page. Prefix log lines with compact local timestamps (`15:04:05`) and the raw message, wrapping long messages to the viewport width. Add lightweight severity coloring for common level markers (`ERROR`, `WARN`, `INFO`, `DEBUG`) to make plain logs easier to scan while preserving the raw message text; use conservative matching near the start of the message or in common fields like `level=error`, `"level":"error"`, `[ERROR]`, or `ERROR `. Auto-follow new events only when the viewport is already at/near the bottom; preserve scroll position when the user has scrolled up. Use `[`/`]` for task-detail tab switching, and use terminal-safe `ctrl+h`/`ctrl+l` in the `Logs` tab to switch the selected container, reset the viewport, and start streaming that container's logs.

## Files to modify
- `go.mod` / `go.sum` — add AWS CloudWatch Logs service module if needed.
- `internal/domain/ecs/types.go` — add log-related domain structs/fields.
- `internal/application/ecs/ports.go` and `internal/application/ecs/service.go` — add validation/sorting/defaults for log retrieval.
- `internal/infrastructure/awsclients/factory.go` — add a cached CloudWatch Logs client.
- `internal/infrastructure/awsecs/service.go` — implement task log discovery/fetching.
- `internal/ui/pages/ecs/ecs.go`, `ecs_commands.go`, `ecs_update.go`, `ecs_keys.go`, `ecs_view.go`, helper/test files — add task-detail tab state, log view state, viewport, commands, keys, rendering, and tests.
- `README.md` — document ECS task log viewing, relevant keys, and required CloudWatch Logs/ECS read permissions.

## Reuse
- Existing ECS selection flow and cancelable async command pattern in `internal/ui/pages/ecs/ecs_commands.go`.
- Existing ECS task detail page/stage pattern in `internal/ui/pages/ecs/ecs_update.go` and `ecs_view.go`.
- Existing Bubble Tea component patterns in the ECS page; add `github.com/charmbracelet/bubbles/viewport` for scrollable log rendering.
- Existing AWS client factory cache/region/profile handling in `internal/infrastructure/awsclients/factory.go`.
- Existing ECS task definition/container metadata available via `DescribeTasks`; will add `DescribeTaskDefinition` to resolve `awslogs` config.

## Steps
- [x] Decide the first supported log source and UX scope.
- [x] Add CloudWatch Logs client support to the AWS client factory.
- [x] Add ECS log domain types and split application service methods: `DescribeTaskLogTargets(ctx, profile, region, taskDefinitionARN, taskID)` and `FetchTaskLogEvents(ctx, profile, region, target, nextToken/lookback/limit)`.
- [x] In AWS infrastructure, resolve container log config from task definition (`awslogs-group`, `awslogs-region`, `awslogs-stream-prefix`) and derive the stream name as `<awslogs-stream-prefix>/<container-name>/<task-id>` for the selected task/container; treat missing stream prefix as an unsupported/friendly empty state.
- [x] Cache resolved task-definition log config in ECS page/session state keyed by task definition ARN, and reset it when changing task/cluster/session.
- [x] Fetch recent log events with CloudWatch Logs `GetLogEvents` using an initial 15-minute lookback capped at 500 events, and preserve the next-forward token for follow-up reads.
- [x] Add a cancellable polling command that re-runs every 3 seconds only while the selected task's `Logs` tab is actively viewed, appending only newer events and always stopping when the user leaves logs/task detail/ECS page focus or when CloudWatch Logs returns an AWS error.
- [x] Add ECS UI state for task-detail tabs (`Overview`, `Logs`), task log loading/errors/events, selected log container index/name, next token, stream/poll active state, and a log viewport.
- [x] Render log events into the viewport with compact local timestamp prefixes (`15:04:05`), lightweight severity coloring, wrapped long messages, responsive viewport sizing, and auto-follow only when the user is already at/near the bottom; preserve scroll position when reviewing older lines.
- [x] Add keybindings/help text for switching task-detail tabs with `[`/`]`, switching log containers with `ctrl+h`/`ctrl+l`, scrolling logs, leaving log streaming, and returning from task detail to the tasks list. Do not add a separate manual log refresh key in v1 because streaming polls automatically.
- [x] Add AWS infrastructure unit tests for `awslogs` config extraction and stream-name derivation from ECS task definitions.
- [x] Add application/UI unit tests using fake service data for validation, task-detail tab transitions, log loading states, container switching, viewport rendering, auto-follow behavior, and error/empty states.
- [x] Update README feature/navigation docs for ECS task log viewing, task detail tabs, log scrolling, container switching keys, and required permissions such as `logs:GetLogEvents` plus ECS task-definition read access.

## Verification
- `go test ./...`
- Manual run: `go run ./cmd/aws-terminal`, authenticate/select profile+region, open ECS cluster, select a running task, open logs, verify loading/error states and displayed recent log lines.
- Manual checks for tasks without `awslogs` config and for IAM-denied CloudWatch Logs permissions.

## Open Questions
1. Answered: first version supports only standard ECS `awslogs` CloudWatch Logs.
2. Answered: task detail should become tabbed, with existing metadata under `Overview` and CloudWatch logs under `Logs`.
3. Answered: logs should load automatically when the `Logs` tab opens, then behave like a stream by polling CloudWatch Logs for newer events every 3 seconds until the user leaves the logs tab.
4. Answered: use a `bubbles/viewport` for the logs view so output is scrollable and constrained to the page.
5. Answered: auto-follow new log lines only when already at/near bottom; preserve scroll position if the user scrolls up.
6. Answered: initial log history should fetch the last 15 minutes, capped at 500 events, to avoid overloading the TUI.
7. Answered: log lines should include compact local timestamps (`15:04:05`) by default.
8. Answered: for multi-container tasks, show logs for one selected container at a time rather than merging streams.
9. Superseded by question 10.
10. Answered: avoid key conflicts by using `[`/`]` for task-detail tabs and `ctrl+h`/`ctrl+l` for previous/next log container in the `Logs` tab; show the selected container in the header and reset/reload streaming when it changes.
11. Answered: if a container has no `awslogs` config, show a friendly empty state with the container name rather than treating it as an error.
12. Answered: if CloudWatch Logs returns an AWS error, stop streaming and show the error until the user changes container or re-enters the Logs tab.
13. Answered: logs in this first pass apply only to currently visible non-stopped tasks; do not add stopped-task browsing yet.
14. Answered: no separate manual refresh key for logs in v1; automatic polling is enough, and retry can happen by changing container or re-entering the Logs tab.
15. Answered: always stop log polling when the user leaves the active ECS task `Logs` view, including losing ECS page focus; restart only when opening the logs tab again.
16. Answered: wrap long log messages to the viewport width rather than requiring horizontal scrolling.
17. Answered: add lightweight severity colors for common log level markers (`ERROR`, `WARN`, `INFO`, `DEBUG`) because plain white logs are hard to scan.
18. Answered: severity coloring should use conservative matching near the start of the message or common structured fields, not arbitrary word matches anywhere in the line.
19. Answered: do not add log search/filter in this first version; handle it in a separate future run.
20. Answered: update README/navigation docs for the new ECS task log view and keys.
21. Answered: add both AWS SDK-level tests for log config/stream derivation and fake-service UI/application tests for behavior.
22. Answered: cache resolved task-definition log config in ECS page/session state keyed by task definition ARN; reset when changing task/cluster/session.
23. Answered: split the log API into `DescribeTaskLogTargets` and `FetchTaskLogEvents` rather than one combined method.
24. Answered: derive ECS awslogs streams using `<awslogs-stream-prefix>/<container-name>/<task-id>`; missing stream prefix should become a friendly unsupported/no-logs state.
25. Answered: README should mention that task logs require CloudWatch Logs permissions such as `logs:GetLogEvents` and ECS task-definition read access.
