# aws-terminal

[![CI](https://github.com/ShAd0W20/aws-terminal/actions/workflows/ci.yml/badge.svg)](https://github.com/ShAd0W20/aws-terminal/actions/workflows/ci.yml)
[![Release](https://github.com/ShAd0W20/aws-terminal/actions/workflows/release.yml/badge.svg)](https://github.com/ShAd0W20/aws-terminal/actions/workflows/release.yml)
[![Latest release](https://badgen.net/github/release/ShAd0W20/aws-terminal)](https://github.com/ShAd0W20/aws-terminal/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A keyboard-first terminal UI for working with AWS resources from your local shell.

`aws-terminal` uses your existing AWS profiles, supports native AWS SSO device-flow login, and provides guided workflows for common AWS tasks without requiring you to remember every CLI command.

## Screenshots

### Dashboard

![aws-terminal dashboard](docs/screenshots/dashboard.png)

### S3 sync review

![aws-terminal S3 sync review](docs/screenshots/s3-review.png)

## Features

- Discover AWS profiles from shared config/credentials files.
- Native AWS SSO OIDC device-flow login without shelling out to the AWS CLI.
- Resolve the active caller identity and region for the selected profile.
- List S3 buckets and sync local files/folders to S3 with an explicit review step.
- Optional S3 delete mode that must be enabled before destructive changes run.
- Static-site-oriented S3 upload metadata, including frontend content-type fallbacks and cache-control presets.
- List CloudFront distributions and create/poll invalidations.
- List/search private ECR repositories, create repositories, and push local Docker images.
- Browse ECS clusters, services, and running tasks with compact service/task detail views that highlight status, IP/network placement, containers, failure reasons, and CloudWatch `awslogs` task logs.
- Browse EC2 instances, view instance details, stop/terminate instances with confirmation, and connect to running instances through AWS Systems Manager Session Manager.
- Browse SQS queues, inspect approximate message counts, view received messages without deleting them, and purge queues with typed confirmation.
- Bubble Tea powered TUI with keyboard navigation and cancellable long-running workflows.

## Install

### macOS and Linux install script

```bash
curl -fsSL https://raw.githubusercontent.com/ShAd0W20/aws-terminal/main/install.sh | bash
```

Install a specific version or location:

```bash
curl -fsSL https://raw.githubusercontent.com/ShAd0W20/aws-terminal/main/install.sh | VERSION=v0.2.0 INSTALL_DIR="$HOME/.local/bin" bash
```

### Homebrew

```bash
brew install ShAd0W20/tap/aws-terminal
```

### Scoop

```powershell
scoop bucket add shadow20 https://github.com/ShAd0W20/scoop-bucket
scoop install aws-terminal
```

### Manual download

Download prebuilt binaries from the [latest GitHub Release](https://github.com/ShAd0W20/aws-terminal/releases/latest).

### Updates

`aws-terminal` checks for new GitHub releases on startup by default and shows a non-blocking prompt when an update is available. Direct macOS/Linux installs can update from the CLI:

```bash
aws-terminal check-update
aws-terminal update
```

Homebrew and Scoop installs should be updated through their package managers:

```bash
brew upgrade aws-terminal
scoop update aws-terminal
```

Set `"checkForUpdatesOnStart": false` in `~/.config/aws-terminal/config.json` to disable automatic startup checks. Manual checks still work.

Published targets:

- Windows amd64
- Linux amd64
- macOS arm64
- macOS amd64

## Quick start

```bash
aws-terminal
```

Or run from source:

```bash
go run ./cmd/aws-terminal
```

The app reads the same AWS configuration files used by the AWS CLI:

- `~/.aws/config`
- `~/.aws/credentials`

Environment overrides such as `AWS_CONFIG_FILE` and `AWS_SHARED_CREDENTIALS_FILE` are also respected.

## Navigation

Global controls:

| Key | Action |
| --- | --- |
| `tab` | Move focus forward through Profiles, Regions, Pages, and the active page workflow |
| `shift+tab` / `backtab` | Move focus backward |
| `↑/↓` or `k/j` | Move within focused lists |
| `enter` | Activate the focused item or continue the current workflow |
| `r` | Refresh profiles from AWS config |
| `q` / `ctrl+c` | Quit |

Pages only receive workflow keys after focusing the Pages pane and pressing `enter`.

## Workflows

### AWS profiles and SSO

- Profiles are loaded from AWS shared config/credentials.
- Non-SSO profiles resolve caller identity with STS.
- SSO profiles use native OIDC device authorization.
- Cached SSO sessions are reused when valid.
- When a new SSO login is required, the app shows the verification URL and one-time code in the TUI.

### S3 sync

The S3 page provides a staged local-to-S3 sync workflow:

1. Select an authenticated profile and region.
2. Open **S3 Buckets**.
3. Select a bucket.
4. Pick a local file or folder.
5. Enter an optional destination prefix.
6. Review uploads/deletes/skips.
7. Optionally enable delete mode.
8. Confirm and run the sync.

Notes:

- Delete is never implicit; it must be enabled from the review screen.
- Delete is disabled for single-file sources.
- Directory sync preserves paths relative to the selected directory.
- Empty prefix means bucket root.
- Uploads refresh content and metadata so static website deployments get updated content types/cache headers.

Useful keys:

| Key | Action |
| --- | --- |
| `enter` | Select/continue/confirm |
| `space` | Toggle delete on the review screen |
| `b` / `esc` | Go back/cancel depending on the stage |
| `i` | After successful sync, jump to CloudFront invalidation |

### CloudFront invalidation

- List distributions for the active profile/region.
- Select a distribution.
- Enter one or more paths, for example `/*` or `/assets/* /index.html`.
- Create an invalidation and poll until completion.
- Copy the equivalent AWS CLI command to the clipboard.

### ECR private repositories

- List and search private ECR repositories.
- Create private repositories.
- View existing image tags/digests.
- Discover local Docker images via the Docker Engine API.
- Push a selected or manually entered local image to ECR.

Docker must be running locally for image discovery and push workflows.

### ECS clusters, services, and tasks

- List ECS clusters for the active profile/region.
- Drill into a cluster to browse services and non-stopped tasks.
- Search clusters, services, and tasks from the page workflow.
- Open service details to see deployment health, desired/running/pending counts, network configuration, runtime settings, and identifiers.
- Update an ECS service from its detail view by selecting an available task definition revision for the service family, changing desired task count, or forcing a new deployment. Updates always show a review screen before AWS changes are submitted.
- Open task details to quickly see status, health, private IP, availability zone, runtime, connectivity, containers, and stopped/failure reasons.
- Stop a running ECS task from task details with an editable stop reason and review screen. The stop reason is sent to ECS for audit context.
- Switch task detail tabs between **Overview** and **Logs**. Logs support ECS `awslogs` CloudWatch Logs streams, load the last 15 minutes initially, and continue polling while the Logs tab is actively viewed.
- In the Logs tab, scroll through the viewport with `↑`/`↓` or `k`/`j`, switch task-detail tabs with `[`/`]`, and switch log containers with `ctrl+h`/`ctrl+l`.
- Required IAM permissions for task logs include ECS task-definition read access, such as `ecs:DescribeTaskDefinition`, and CloudWatch Logs access, such as `logs:GetLogEvents`.
- Required IAM permissions for service updates include `ecs:ListTaskDefinitions` and `ecs:UpdateService`, plus any `iam:PassRole` permissions required by the selected task definition. Required IAM permissions for stopping tasks include `ecs:StopTask`.

### EC2 instances

- List and search non-terminated EC2 instances for the active profile/region.
- Open instance details to see status, network placement, runtime metadata, security groups, block devices, network interfaces, tags, and identifiers.
- Stop running instances from the detail view with a review screen.
- Terminate instances from the detail view only after typing the exact instance ID.
- Connect to running instances from the detail view with `c` using AWS Systems Manager Session Manager.

EC2 Session Manager connections shell out to the AWS CLI command `aws ssm start-session`. To use this feature, install both the AWS CLI and the Session Manager plugin locally:

```bash
brew install awscli
brew install --cask session-manager-plugin
session-manager-plugin --version
```

The target instance must also be configured for Session Manager access, including an attached IAM role with SSM permissions and a running SSM Agent.

### SQS queues

- List and search SQS queues for the active profile/region.
- View approximate available and in-flight message counts.
- Pull up to 10 messages for view-only inspection. Messages are not deleted and may remain temporarily in-flight until their visibility timeout expires.
- Purge a queue only after typing the exact queue name.

Useful keys:

| Key | Action |
| --- | --- |
| `enter` | Open queue actions |
| `ctrl+f` | Search queues |
| `p` | Pull messages for view-only inspection |
| `x` | Purge queue with typed confirmation |
| `b` / `esc` | Go back/cancel depending on the stage |

## Safety model

`aws-terminal` is intended to make AWS operations easier without hiding important state transitions:

- Destructive actions use explicit confirmation/review screens.
- S3 delete must be opted into every run.
- Long-running AWS operations are cancellable where practical.
- Async results are scoped to the page/session that started them.
- The app does not store AWS credentials beyond standard AWS SSO token cache behavior.

## Development

Requirements:

- Go matching `go.mod`
- Git
- Optional: Docker for ECR push workflow development

Common commands:

```bash
go run ./cmd/aws-terminal
go test ./...
go build ./...
```

Format Go changes before committing:

```bash
gofmt -w <changed-go-files>
```

## Project structure

```text
cmd/aws-terminal                         # application entrypoint
internal/app                             # dependency wiring and Bubble Tea program bootstrap
internal/domain/*                        # core types only; no UI or AWS SDK imports
internal/application/*                   # use cases and ports/interfaces
internal/infrastructure/*                # AWS SDK, Docker, and filesystem adapters
internal/ui/pageapi                      # shared Page contract and shell/page state
internal/ui/workflow                     # reusable workflow helpers
internal/ui/shell                        # main Bubble Tea shell model/update/view
internal/ui/components                   # shared TUI components
internal/ui/pages/s3                     # S3 workflow page
internal/ui/pages/cloudfront             # CloudFront workflow page
internal/ui/pages/ecr                    # ECR workflow page
internal/ui/pages/ecs                    # ECS cluster/service/task browser page
internal/ui/styles                       # shared Lip Gloss theme helpers
```

Architecture boundaries:

```text
ui -> application -> domain
infrastructure -> application/domain
app wires ui + infrastructure together
```

AWS SDK imports should stay in `internal/infrastructure` and wiring code, not in domain/application/UI packages.

## Releases

CI runs on pushes to `main` and pull requests.

Release builds run when a version tag is pushed:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The release workflow:

1. Runs tests.
2. Builds Windows, Linux, and macOS binaries.
3. Uploads release archives and checksums.
4. Creates GitHub-generated release notes.
5. Updates the Homebrew tap and Scoop bucket when `PACKAGE_REPO_TOKEN` is configured.

Use conventional commit messages to improve generated release notes.

## Contributing

Contributions are welcome. Good first contributions include:

- Bug reports with reproduction steps.
- Documentation improvements.
- Tests for application-layer behavior.
- New AWS workflows that preserve the existing architecture boundaries.

Before opening a pull request, run:

```bash
go test ./...
go build ./...
```

## Security

Please avoid opening public issues for sensitive security reports. If you find a vulnerability, contact the maintainer privately first.

## License

MIT. See [LICENSE](LICENSE).
