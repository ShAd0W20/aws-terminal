# SQS Service Refactor Plan

## Context

The SQS feature exists across application, infrastructure, and UI layers. The current service path works, but there is some small duplication and coupling:

- `MaxCount` fallback is enforced in both `internal/application/sqs/service.go` and `internal/infrastructure/awssqs/service.go`.
- `internal/infrastructure/awssqs/service.go` mixes client lookup, timeouts, AWS calls, and domain mapping in one file.
- The AWS adapter test fake must implement list/attributes/receive/purge even when a test only needs one operation.

Goal: refactor all SQS layers with the smallest useful diff, not redesign the page or add features.

## Approach

Prefer a boring cleanup over new abstractions. Keep the existing public interfaces unless a rename removes confusion. Make `internal/application/sqs` the single validation/defaulting boundary, keep `internal/infrastructure/awssqs` focused on AWS calls plus mapping, and trim the UI page by centralizing repeated “start async operation / clear state / spinner” code where it already exists.

Do not add dependencies, change app wiring, or alter SQS behavior: list queues, pull view-only messages, and purge with exact queue-name confirmation should continue working.

## Files to modify

Likely files:

- `internal/domain/sqs/types.go` — only if field comments/names clarify intent without churn.
- `internal/application/sqs/service.go`
- `internal/application/sqs/service_test.go`
- `internal/infrastructure/awssqs/service.go`
- `internal/infrastructure/awssqs/service_test.go`
- `internal/ui/pages/sqs/sqs.go`
- `internal/ui/pages/sqs/sqs_commands.go`
- `internal/ui/pages/sqs/sqs_update.go`
- `internal/ui/pages/sqs/sqs_helpers.go`
- `internal/ui/pages/sqs/sqs_view.go`
- `internal/ui/pages/sqs/sqs_test.go`

Skip `internal/app/*` unless a compile-time interface change forces it.

## Reuse

- Existing validation helpers in `internal/application/sqs/service.go`: `validateProfileRegion`, `validateQueueActionInput`.
- Existing AWS client timeout/cache helpers in `internal/infrastructure/awsclients/factory.go`: `WithTimeout`, `OperationTimeout`, `SQS`.
- Existing AWS mapping helpers in `internal/infrastructure/awssqs/service.go`: `queueName`, `parseAttributeInt`, `parseUnixMilliseconds`.
- Existing tests in `internal/application/sqs/service_test.go`, `internal/infrastructure/awssqs/service_test.go`, and `internal/ui/pages/sqs/sqs_test.go`.
- Existing SQS page helpers in `internal/ui/pages/sqs/sqs_helpers.go`: `resetForSession`, `cancelAll`, `filteredQueues`, `currentQueue`, `syncTable`.
- Existing SQS command creators in `internal/ui/pages/sqs/sqs_commands.go`: `loadQueuesCmd`, `receiveMessagesCmd`, `purgeQueueCmd`.

## Steps

- [x] Confirm scope: all SQS layers.
- [ ] Application layer: make `ReceiveMessages` the only place that defaults invalid `MaxCount` to 10; keep profile/region/queue trimming in `validateQueueActionInput`.
- [ ] Application tests: add/adjust coverage proving queue actions trim inputs and clamp invalid `MaxCount` to 10 before delegating.
- [ ] Infrastructure layer: remove duplicated `MaxCount` fallback from `ReceiveMessages`; trust the application service boundary.
- [ ] Infrastructure layer: extract tiny mapping helpers only where they shorten `service.go` (for example message mapping); avoid new files unless the file clearly gets easier to read.
- [ ] Infrastructure tests: keep the fake client; only split it if the refactor would otherwise make tests noisier.
- [ ] UI layer: reduce repeated async-start boilerplate in `updateQueueActionsStage`, `updateMessagesStage`, `updatePurgeConfirmStage`, and `startQueueRefresh` with small page methods such as `startMessagePull`, `startPurge`, and `startQueueRefresh`.
- [ ] UI layer: keep the existing stages and keys; do not add new actions or change confirmation behavior.
- [ ] UI tests: keep current behavior tests and add only one regression check if a helper changes state transitions.
- [ ] Keep public method names and app wiring unchanged to avoid churn.

## Verification

- [ ] Run `gofmt -w` on changed Go files.
- [ ] Run `go test ./internal/application/sqs ./internal/infrastructure/awssqs ./internal/ui/pages/sqs`.
- [ ] Run `go test ./...` before handoff.
- [ ] Manual smoke check if available: open SQS page, refresh queues, pull messages view-only, and verify purge still requires exact queue name.
