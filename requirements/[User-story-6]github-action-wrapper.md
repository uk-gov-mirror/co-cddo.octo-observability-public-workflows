# Story: Backwards-Compatible GitHub Action Wrapper

## [STORY-002-004] Backwards-Compatible GitHub Action Wrapper for Docker Image

### Background

Existing consumers of the current GitHub Action (STORY-001-001/002) should not be forced to change their workflow files when the underlying implementation migrates to the Docker image. This story creates a thin GitHub Action wrapper that delegates to the new Docker image, maintaining the same action inputs, outputs, and behaviour. Existing users get the benefits of the new architecture (multi-platform image, improved auth options) without any migration effort.

### Business Value

- Protect existing customers from breaking changes — their workflows continue to work without modification
- Maintain GitHub Actions as a first-class, ergonomic experience (native action inputs/outputs, marketplace listing continuity)
- Allow existing users to opt into new features (OIDC auth, different source platforms) incrementally via new optional inputs
- Preserve trust and adoption metrics associated with the existing action version

### Dependencies and Assumptions

- **Prerequisites**: STORY-002-001 (core Docker image) is complete and published to ghcr.io; ideally STORY-002-002 (OIDC auth) is also complete so the wrapper can expose OIDC as an option
- **Data assumptions**: The existing action's input interface (api-url, service-id, api-token) is the contract to maintain; the Docker image accepts equivalent configuration via environment variables or flags
- **Integration points**: Docker image on ghcr.io (run), GitHub Actions runtime (action.yml), existing user workflow files (backwards compatibility)
- **Business constraints**: The action must remain at the same repository path and major version tag so existing `uses: co-cddo/octo-observability-public-workflows@v1` references continue to resolve

### Scope In

- New `action.yml` using Docker container action type (referencing the published image)
- All existing action inputs preserved with identical names and semantics
- New optional inputs for features introduced by the Docker image (e.g., `auth-mode`, `platform`)
- All existing action outputs preserved with identical names and semantics
- GitHub Actions OIDC token automatically passed to the container when `auth-mode: oidc` is selected
- Clear migration guide documenting what changed under the hood and new optional capabilities
- Version tag continuity — existing `@v1` references work, new major version not required

### Scope Out

- Changing the existing action's repository URL or namespace
- Removing or renaming any existing inputs/outputs (breaking changes)
- Implementing new functionality in the wrapper itself (all logic lives in the Docker image)
- Supporting the action on GHES versions that don't support Docker container actions
- GitLab CI or Jenkins wrapper scripts (those CI systems use `docker run` directly)

### Acceptance Criteria

#### AC1: Existing workflow continues to work without changes

**Given** a customer's existing workflow file that uses the action with inputs: `api-url`, `service-id`, and `api-token`
**When** the action is updated to the new wrapper version under the same `@v1` tag
**Then** the workflow executes successfully, the SBOM is retrieved and submitted identically to the previous implementation, and no workflow file changes are required

#### AC2: New OIDC auth option available via optional input

**Given** a customer who wants to use OIDC instead of an API key
**When** they add `auth-mode: oidc` to their action inputs and configure `permissions: id-token: write`
**Then** the action passes the GitHub Actions OIDC token to the Docker container, which authenticates via token exchange and submits the SBOM successfully

#### AC3: Action failure surfaces clearly in GitHub Actions UI

**Given** the Docker container exits with a non-zero exit code (e.g., due to an invalid token)
**When** the GitHub Actions runner receives the failure
**Then** the action step is marked as failed in the workflow run UI, the error message from the container is visible in the step logs, and the workflow run status reflects the failure

#### AC4: Action outputs remain available to downstream steps

**Given** a customer's workflow that uses the action's outputs (e.g., submission status, SBOM ID) in subsequent steps
**When** the Docker container completes successfully and produces output values
**Then** the action exposes the same outputs with the same names as the previous version, and downstream steps can reference them via `${{ steps.sbom.outputs.<name> }}`

#### AC5: Version tag compatibility

**Given** a customer referencing the action as `uses: co-cddo/octo-observability-public-workflows@v1`
**When** the wrapper is released
**Then** the `v1` tag points to the new wrapper, and existing workflows resolve and execute the updated action without pinning to a new version

#### Non-Functional Expectations

- The wrapper adds no more than 10 seconds of overhead compared to running the Docker image directly (time for GitHub Actions to pull the image and set up the container action)
- The action.yml file must validate successfully with GitHub's action metadata schema
