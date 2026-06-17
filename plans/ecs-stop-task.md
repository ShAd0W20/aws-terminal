# ECS Stop Task Plan

## Context

Add a TUI workflow that lets a user stop a selected ECS task from the task detail screen. This is a mutating ECS action, so it should follow the app's safety model and require an explicit review/confirmation screen before calling AWS.

Initial code scan findings:
- ECS task detail already exists in `internal/ui/pages/ecs/ecs_update.go` and `internal/ui/pages/ecs/ecs_view.go` under `ecsStageTaskDetail`.
- Existing ECS update-service workflow provides a reusable pattern for domain input/result types, application validation, AWS adapter call, staged UI review, success/error messages, and refresh after mutation.
- Task state already includes `Task.ARN`, `Task.ID`, `LastStatus`, `DesiredStatus`, `StoppedReason`, service/group metadata, and task definition information in `internal/domain/ecs/types.go`.
- ECS page already refreshes tasks for a cluster with `loadTasksCmd` after mutating service updates.

## Approach

Recommended approach: add a stop-task workflow that starts from task detail, shows a review screen with exact cluster/task identifiers and stop reason, then calls ECS `StopTask`, refreshes the task list, and returns to a safe detail/resources state with a success message.

Resolved decisions:
- Stop reason will be editable, prefilled with the short default `Stopped from aws-terminal`, and included on the confirmation screen before calling AWS. Extra profile/region/task context belongs in the review UI and CloudTrail, not the stop reason. README should document `ecs:StopTask` and mention the entered stop reason is sent to ECS for audit context.
- Stop action will be allowed for any ECS launch type, but only offered in the UI for tasks that appear stoppable and have both cluster ARN and task ARN available; tasks with `LastStatus` of `STOPPED`, `STOPPING`, or `DEPROVISIONING` will be blocked in the UI and validation path. Launch type will be displayed on the review screen for context, not used as a blocker. Domain/application input should model AWS semantics by accepting either task ID or ARN as `Task`, while the UI passes ARN when available.
- After a successful stop request, return to the same task detail screen, show a short auto-clearing success banner, and refresh cluster services/tasks. If the task disappears from the refreshed list, preserve the selected task detail instead of abruptly navigating away. When the refreshed task list still contains the task, preserve task-table selection by ARN; otherwise fall back to the nearest valid row.
- Use `x` as the task-detail key to start the stop-task workflow, shown in footer/help only when the selected task appears stoppable and the ECS page is focused. For non-stoppable tasks, show status context in the task detail body but do not show the stop key. Initial version exposes stop only from task detail, not directly from the tasks table, so users inspect task context before mutating. Starting a stop-task workflow clears previous mutation success/error banners to avoid stale messaging.
- Prefer shared ECS mutation feedback state (`mutationSuccess`, `mutationErr`, `mutationSuccessSeq`) for service update and stop-task banners/errors if the refactor remains small; otherwise keep stop-specific state to avoid over-expanding scope.
- Use two stop-task stages before mutation: a single-line editable reason-entry screen, then a read-only review screen. Confirmation will be the existing review-screen pattern: press `Enter` to submit, `b`/Esc to go back/cancel. No typed confirmation phrase is required. Add explicit stages such as `ecsStageStopTaskReason`, `ecsStageStopTaskReview`, and `ecsStageStoppingTask`; do not reuse service-update stages.
- Stop task will be available from both task Overview and Logs tabs. Entering the stop workflow will stop log streaming and remember the originating tab. Cancel/back returns to the originating tab; successful stop returns to task Overview to avoid restarting logs for a task being stopped.
- Stop reason is required after trimming whitespace so every stop request has an audit trace. Blank reasons keep the user on the reason screen with an inline validation error.
- Review screen will include task identifiers, launch type, task definition, group, stop reason, and a compact container summary with names/statuses and shortened images when available.
- Review screen will warn when the task appears service-managed (`Group` starts with `service:`) that ECS may launch a replacement task to maintain desired count.
- StopTask result will update `selectedTask` immediately before refreshing cluster services and tasks, so the detail screen shows instant feedback and then reconciles with AWS refresh data. Reuse the existing AWS `taskFromSDK` mapper for the returned task so stop results and listed tasks stay consistent.
- If AWS returns an error such as task not found/stale selection, keep the user on the review screen, show the AWS error, and automatically refresh services/tasks to reconcile UI state.
- During the in-flight stopping state, `b`/Esc cancels the local request context and returns to review. UI copy should frame this as canceling the wait/request context, not guaranteed rollback after AWS receives the request.
- For this implementation, keep the stop-task confirmation flow local to the ECS page, but record a follow-up architectural improvement to introduce a generic confirmation modal/style notification for future mutating actions.

Open design decisions are being resolved one at a time with the user.

## Files to modify

Likely critical paths:
- `internal/domain/ecs/types.go`
- `internal/application/ecs/ports.go`
- `internal/application/ecs/service.go`
- `internal/application/ecs/service_test.go`
- `internal/infrastructure/awsecs/service.go`
- `internal/infrastructure/awsecs/service_test.go`
- `internal/ui/pages/ecs/ecs.go`
- `internal/ui/pages/ecs/ecs_commands.go`
- `internal/ui/pages/ecs/ecs_keys.go`
- `internal/ui/pages/ecs/ecs_update.go`
- `internal/ui/pages/ecs/ecs_stop_task.go` (new, for stop-task-specific handlers)
- `internal/ui/pages/ecs/ecs_view.go`
- `internal/ui/pages/ecs/ecs_update_test.go`
- `internal/ui/pages/ecs/ecs_view_test.go`
- `README.md`

## Reuse

- Reuse update-service mutation pattern in `internal/ui/pages/ecs/ecs_update.go` for confirmation, spinner, cancellation, success/error handling, and post-mutation refresh.
- Reuse ECS task detail rendering and selected task state in `internal/ui/pages/ecs/ecs_view.go`.
- Reuse application service validation style in `internal/application/ecs/service.go`.
- Reuse AWS ECS client construction in `internal/infrastructure/awsecs/service.go`.
- Reuse `taskFromSDK` in `internal/infrastructure/awsecs/service.go` to map the `StopTask` response.

## Steps

- [x] Resolve UX and safety decisions with user.
  - [x] Stop reason is editable and prefilled with the short default `Stopped from aws-terminal`.
  - [x] Any launch type is allowed; only tasks with cluster ARN, task ARN, and non-stopped/non-stopping statuses expose the stop workflow.
  - [x] Successful stop returns to task detail, shows a brief auto-clearing success banner, and refreshes cluster services/tasks.
  - [x] Stop-task workflow starts from task detail with `x`; no tasks-table shortcut in the initial version.
  - [x] Stop workflow uses reason-entry then read-only review; review confirmation uses `Enter` with no typed phrase.
  - [x] Stop task is available from both task detail tabs, stops log streaming when opened, cancel returns to originating tab, success returns to Overview.
  - [x] Stop reason is required to preserve an audit trace.
  - [x] Review shows compact container summary and warns that service-managed tasks may be replaced.
  - [x] Use StopTask response to update selected task immediately, then refresh services and tasks.
  - [x] StopTask AWS errors remain on review and trigger services/tasks refresh.
  - [x] In-flight stop request supports local cancellation with `b`/Esc.
  - [x] Generic confirmation modal is desired as a later reusable UI improvement; stop-task can remain local for now.
- [x] Add stop-task domain input/result types.
- [x] Extend ECS application port and service validation.
- [x] Implement AWS `ecs.StopTask` adapter.
- [x] Add task detail stop key binding and review/confirmation stages, with stop-task-specific handlers in a new `ecs_stop_task.go` file.
- [x] Refresh task list after stop succeeds and display a success message.
- [x] Update docs/IAM permissions with `ecs:StopTask` and stop-reason audit context.
- [x] Add application, infrastructure, and UI tests, including safety tests that stopped/stopping/deprovisioning tasks do not show or start the stop workflow.

## Future Improvements

- Introduce a reusable generic confirmation modal/style notification for mutating actions across the TUI. This should eventually replace local per-page review screens where appropriate, but should be designed separately so the stop-task feature is not blocked by a cross-app component refactor.

## Verification

- Run focused ECS application, infrastructure, and UI tests.
- Run `go test ./...`.
- Manual TUI check: open ECS cluster → task detail → stop task → review → confirm → observe AWS request, success message, and refreshed tasks.
