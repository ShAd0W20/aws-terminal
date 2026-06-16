# ECS Update Service Plan

## Context

Add an ECS workflow to update a service within a selected cluster. The user should be able to update:

- Task definition
- Desired task count
- Force new deployment

Initial scan shows the app is a Go Bubble Tea AWS TUI with existing ECS browsing support across domain, application, infrastructure, and UI layers.

## Approach

Add the feature vertically through the existing ECS layers:

- Extend the ECS domain with task-definition summary plus update-service input/result models.
- Extend the application ECS port and service with validation/normalization for listing task definitions and updating services.
- Implement task-definition loading with AWS SDK `ListTaskDefinitions`, scoped to the selected service's current task-definition family, and implement the update call with AWS SDK `UpdateService`.
- Add a staged ECS UI workflow from service detail: select from available task definitions, edit desired count, toggle force-new-deployment, then always show a review/confirmation screen before executing.
- Refresh services and tasks for the selected cluster after a successful update so the table/detail views reflect the new deployment.

The UI should reuse the current Bubble Tea staged-page pattern already used by S3/ECR rather than introducing a new workflow framework.

## Files to modify

Likely candidates:

- `internal/domain/ecs/types.go`
- `internal/application/ecs/ports.go`
- `internal/application/ecs/service.go`
- `internal/infrastructure/awsecs/service.go`
- `internal/ui/pages/ecs/ecs.go`
- `internal/ui/pages/ecs/ecs_commands.go`
- `internal/ui/pages/ecs/ecs_keys.go`
- `internal/ui/pages/ecs/ecs_update.go`
- `internal/ui/pages/ecs/ecs_view.go`
- ECS tests under `internal/application/ecs`, `internal/infrastructure/awsecs`, and `internal/ui/pages/ecs`

## Reuse

- Existing ECS vertical layering:
  - UI service interface and ECS state live in `internal/ui/pages/ecs/ecs.go`.
  - Async commands/cancellation helpers live in `internal/ui/pages/ecs/ecs_commands.go`.
  - State transitions and key handling live in `internal/ui/pages/ecs/ecs_update.go`.
  - Detail rendering lives in `internal/ui/pages/ecs/ecs_view.go`.
  - Application validation/sorting lives in `internal/application/ecs/service.go`.
  - AWS SDK mapping lives in `internal/infrastructure/awsecs/service.go`.
- Existing detail view fields already expose the current service task definition ARN/name and desired count in `domain/ecs.Service`, so the update form can prefill from `selectedService`.
- Existing `taskDefinitionName`/ARN parsing helpers in `internal/infrastructure/awsecs/service.go` can be reused or generalized to derive the task-definition family for `ListTaskDefinitions`.
- Existing table/list navigation patterns in the ECS service/task tables can be reused for a task-definition select list; no free-text task definition input should be added.
- Existing `textinput.Model` usage in ECR (`internal/ui/pages/ecr/ecr.go`) and S3 (`internal/ui/pages/s3/s3_update.go`) provides the desired-count input/focus pattern to reuse.
- Existing cancellable command pattern in `internal/ui/pages/ecs/ecs_commands.go` should be reused for the update request.
- Existing status construction helpers in `internal/ui/workflow/workflow.go` can be reused/extended for loading/error/success messaging.

## Steps

- [x] Add `TaskDefinitionSummary` plus `UpdateServiceInput`/`UpdateServiceResult` domain types. `TaskDefinitionSummary` should include ARN, display name (`family:revision`), family, revision, and status if available. Use `*int` for desired count and a string/ARN for selected task definition so unchanged fields can be omitted; use `bool` for force-new-deployment.
- [x] Extend `internal/application/ecs.API`, `Service`, and tests with `ListTaskDefinitions(ctx, profile, region, familyPrefix)` validation:
  - require profile and family prefix;
  - trim all strings;
  - sort newest/highest revision first if not already returned that way.
- [x] Extend `internal/application/ecs.API`, `Service`, and tests with `UpdateService(ctx, input)` validation:
  - require profile, cluster ARN, and service name/ARN;
  - require at least one requested change: task definition, desired count, or force-new-deployment;
  - reject negative desired counts;
  - trim profile/region/cluster/service/task definition strings.
- [x] Implement `internal/infrastructure/awsecs.Service.ListTaskDefinitions` with AWS SDK `ecs.ListTaskDefinitionsPaginator`:
  - derive `familyPrefix` from the selected service's current task definition family in the UI before calling;
  - request active task definitions sorted descending where supported;
  - map ARNs to `TaskDefinitionSummary` values; keep current task definition in the list even if AWS returns it separately or it is not first.
- [x] Implement `internal/infrastructure/awsecs.Service.UpdateService` with AWS SDK `ecs.UpdateServiceInput`:
  - `Cluster`, `Service`, `TaskDefinition`, `DesiredCount`, and `ForceNewDeployment` mapped from the domain input;
  - return the updated `domain/ecs.Service` via existing `serviceFromSDK` mapping.
- [x] Extend ECS UI state in `internal/ui/pages/ecs/ecs.go`:
  - add stages such as `ecsStageUpdateTaskDefinition`, `ecsStageUpdateDesiredCount`, `ecsStageUpdateReview`, and `ecsStageUpdating`;
  - add task-definition loading/error state, a selected task-definition index, and a paginator/table or list model for the select/dropdown-style chooser;
  - add a text input only for desired count;
  - add force-new-deployment toggle, update error/success fields, and cancellation state.
- [x] Add a service-detail key binding, likely `u`, to start editing the selected service. On start, derive the current task-definition family, load available task definitions for that family, and preselect `selectedService.TaskDefinitionARN` when present. Prefill desired count from `selectedService.DesiredCount`.
- [x] Implement staged update navigation in `ecs_update.go`:
  - task definition screen is selection-only; no free text entry;
  - up/down/page keys choose an available task definition, and the current task definition is preselected;
  - desired count parses a non-negative integer;
  - force-new-deployment toggles on review;
  - Enter on review confirms and starts the update; every mutating update must pass through this confirmation screen;
  - `b`/`Esc` backs out/cancels without changes.
- [x] Add `updateServiceCmd` in `ecs_commands.go`; on success update `selectedService`, refresh services and tasks for the selected cluster, and show a short success message.
- [x] Render task-definition select, desired-count input, and review screens in `ecs_view.go`, including the exact cluster/service, old/new task definition, old/new desired count, and force-new-deployment status.
- [x] Update README ECS section and IAM permissions to mention `ecs:UpdateService`.
- [x] Add/extend tests for application validation, AWS mapping where practical, and ECS UI state transitions/view text.

## Verification

- Run `go test ./...`.
- Manual TUI check with an ECS test service:
  1. Select profile and region.
  2. Open ECS, select a cluster, then a service.
  3. Press `u`, change task definition and desired count, toggle force-new-deployment, and confirm.
  4. Verify service details refresh with the new task definition/desired count and deployments show a new rollout.
  5. Repeat with only force-new-deployment enabled to ensure a redeploy can run without changing task definition or desired count.
- AWS permissions needed for manual verification: existing ECS read permissions plus `ecs:ListTaskDefinitions`, `ecs:UpdateService`, and any IAM pass-role permissions required by the selected task definition.

## Decisions

- Task definitions are selected from loaded options for the selected service's current task-definition family; users cannot enter free text.
- The task-definition selection and desired count are always prefilled from the selected service.
- Every service update goes through a review/confirmation screen before calling AWS.
