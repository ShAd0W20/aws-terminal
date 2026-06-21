# SQS Page Plan

## Context

Add SQS as the next AWS service page in `aws-terminal`. For the initial version, the page should visualize each SQS queue and show message count metrics: available messages and in-flight messages.

## Approach

- Follow the existing layered pattern used by S3/ECR/ECS/CloudFront: domain types, application service/ports, AWS infrastructure adapter, UI page, and app registration.
- Keep the first visible SQS page read-only: list queue URLs for the selected profile/region, fetch queue attributes, and render exactly three columns: queue name, available messages, and in-flight messages.
- Use SQS queue attributes `ApproximateNumberOfMessages` for available messages and `ApproximateNumberOfMessagesNotVisible` for in-flight messages. Do not show delayed messages, queue URL, or ARN in this first view.
- Structure the SQS domain/service/page so future actions can be added cleanly later, such as purge queue, pull messages, and send test messages, without implementing those actions now.
- Make the page auto-load on active profile/region changes, support refresh, search/filter, keyboard up/down navigation, loading/error states, and cancellation using the same page message ownership/session-key pattern as existing pages.

## Files to modify

- `go.mod` / `go.sum` — add `github.com/aws/aws-sdk-go-v2/service/sqs`.
- `internal/domain/sqs/types.go` — new queue summary model.
- `internal/application/sqs/ports.go` — new read-only queue API port.
- `internal/application/sqs/service.go` — new service for validation and sorting.
- `internal/infrastructure/awsclients/factory.go` — add cached SQS SDK client constructor.
- `internal/infrastructure/awssqs/service.go` — AWS adapter using `ListQueues` and `GetQueueAttributes`.
- `internal/ui/pages/sqs/*` — new SQS page files following the existing page package split.
- `internal/app/app.go` and `internal/app/pages.go` — construct/register SQS service/page.
- Tests under new/modified packages, especially `internal/application/sqs` and `internal/ui/pages/sqs`.

## Reuse

- Page contract and state: `internal/ui/pageapi/page.go`.
- Page registration pattern: `internal/app/pages.go` and app construction in `internal/app/app.go`.
- AWS client cache/timeout helpers: `internal/infrastructure/awsclients/factory.go` (`WithTimeout`, `OperationTimeout`, `NormalizeRegion`, `CacheKey`).
- Read-only list-and-load page flow: `internal/ui/pages/cloudfront/*` and `internal/ui/pages/ecr/ecr_commands.go`/`ecr_update.go`.
- Styling/table helpers: `internal/ui/styles/*`, `internal/ui/tableutil/tableutil.go`, and table usage in `internal/ui/pages/ecr/ecr_view.go`.

## Steps

- [x] Add `domain/sqs.Queue` with `Name`, `URL`, `ARN`, `AvailableMessages`, and `InFlightMessages`; keep `URL`/`ARN` in the model for future SQS actions but do not render them in the initial table.
- [x] Add `application/sqs.QueueAPI` with `ListQueues(ctx, profileName, region string) ([]domainsqs.Queue, error)` plus a service that trims/validates profile + region, delegates to the API, and sorts queues by name.
- [x] Add SQS support to `awsclients.Factory`: import AWS SDK SQS package, add `sqsClients` cache map, initialize it, and expose `SQS(ctx, profileName, region)`.
- [x] Implement `infrastructure/awssqs.Service.ListQueues`:
  - Use `ListQueuesPaginator` or repeated `ListQueues` calls to collect queue URLs.
  - For each URL, call `GetQueueAttributes` for `QueueArn`, `ApproximateNumberOfMessages`, and `ApproximateNumberOfMessagesNotVisible`.
  - Parse queue name from URL as a fallback if an ARN is unavailable.
  - Convert missing/malformed numeric attributes to zero rather than failing the whole page unless the AWS call itself fails.
- [x] Add `ui/pages/sqs` page package:
  - Read-only page with `SQSService` interface, `queuesLoadedMsg`, session key, loading/error fields, queue slice, selected index, search input, spinner, and cancel function.
  - Auto-load from `OnStateChanged` when active session/profile/region changes.
  - Support `r` refresh, `ctrl+f` search, `esc` to blur/cancel, and up/down navigation.
  - Render a compact table with only `Queue`, `Available`, and `In flight` columns.
  - Keep selected queue state and internal queue URL/ARN available for later action stages, but expose no destructive or message actions in this first version.
- [x] Register SQS in app wiring: construct `appsqs.NewService(awssqs.NewService())`, pass it through `DefaultPages`, and append `sqspage.NewSQSPage(...)` to the page registry.
- [x] Add focused tests:
  - Application service sorts queues and rejects missing profile/region.
  - UI page loads on state change, filters by search, refreshes, and displays available/in-flight counts.
  - AWS adapter conversion helpers parse queue names/count attributes correctly without requiring live AWS.

## Verification

- [x] Run `go test ./internal/application/sqs ./internal/ui/pages/sqs ./internal/infrastructure/awssqs ./internal/infrastructure/awsclients ./internal/app`.
- [x] Run `go test ./...`.
- [ ] Manual check: launch app, select SQS page, verify queues appear with available and in-flight counts for selected profile/region.
- [ ] Manual refresh check: send/receive a test message in AWS, press refresh, and confirm available/in-flight counts update according to AWS approximate SQS metrics.
