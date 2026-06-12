# Story: GitLab SBOM Retrieval

## [STORY-002-003] GitLab SBOM Retrieval Support

### Background

Many organisations use GitLab as their source control platform. GitLab provides dependency information via its Dependency List API and supports CycloneDX SBOM export. To serve these customers, the SBOM retriever must be extended to retrieve SBOMs from GitLab projects — not just GitHub repositories. This story adds GitLab as a second supported source control platform, validating the multi-platform architecture established in STORY-002-001.

### Business Value

- Unlock GitLab-hosted organisations as customers of the SBOM observability platform
- Demonstrate the retriever's multi-platform capability, making the product viable for organisations with mixed source control estates
- Enable a single tool for customers who have repositories on both GitHub and GitLab

### Dependencies and Assumptions

- **Prerequisites**: STORY-002-001 (core Docker image with GitHub support) is complete; the image's architecture supports adding new source control platforms without restructuring
- **Data assumptions**: Target GitLab projects have dependency scanning configured or the Dependency List API is available (GitLab Ultimate/Premium, or self-managed with the feature enabled)
- **Integration points**: GitLab Dependency List API or CycloneDX export endpoint (read), SBOM ingestion API (write)
- **Business constraints**: Must support both GitLab.com (SaaS) and self-managed GitLab instances via configurable base URL

### Scope In

- New platform flag (e.g., `--platform gitlab`) to select GitLab as the source
- GitLab project identification via project ID or path (e.g., `--project group/subgroup/project`)
- GitLab personal access token or project token authentication for API access
- Retrieve SBOM/dependency data from GitLab's API and normalise to the format expected by the ingestion API
- Support for configurable GitLab instance URL (default: `https://gitlab.com`)
- Clear error messages for GitLab-specific failure modes (feature not enabled, insufficient token scope)

### Scope Out

- GitLab CI-specific OIDC integration for the source control token (users provide a GitLab token directly)
- Bitbucket, Azure DevOps, or other source control platforms (future stories)
- Merge request / pipeline-triggered automatic submissions (CI integration is the user's responsibility)
- GitLab group-level SBOM aggregation (single project per run)

### Acceptance Criteria

#### AC1: Successful SBOM retrieval from GitLab.com project

**Given** a GitLab.com project "my-group/my-project" with dependency scanning results available, and a valid GitLab personal access token with `read_api` scope
**When** a user runs `docker run ghcr.io/co-cddo/sbom-retriever --platform gitlab --project my-group/my-project --gitlab-token <token> --api-key <key> --api-url https://ingest.example.com`
**Then** the container retrieves the dependency/SBOM data from GitLab, submits it to the ingestion API, exits with code 0, and logs a success message including the project path

#### AC2: Successful SBOM retrieval from self-managed GitLab instance

**Given** a self-managed GitLab instance at `https://gitlab.internal.company.com` with a project that has dependency data
**When** a user runs the container with `--platform gitlab --gitlab-url https://gitlab.internal.company.com --project 42 --gitlab-token <token>`
**Then** the container connects to the self-managed instance, retrieves the SBOM, and submits it successfully

#### AC3: GitLab project without dependency scanning enabled

**Given** a GitLab project that does not have dependency scanning configured or the feature is not available on the instance's licence tier
**When** the container attempts to retrieve the SBOM
**Then** it exits with a non-zero exit code and logs a clear message explaining that dependency data is not available for this project, with guidance on enabling dependency scanning

#### AC4: Invalid or insufficient GitLab token

**Given** a GitLab token that is expired or lacks the `read_api` scope
**When** the container attempts to authenticate with GitLab
**Then** it exits with a non-zero exit code and logs an error indicating authentication failed or insufficient permissions, without exposing the token value

#### AC5: GitHub retrieval continues to work unchanged

**Given** an existing GitHub-based workflow using `--platform github` (or the default platform)
**When** the user runs the container with their existing GitHub configuration
**Then** the behaviour is identical to STORY-002-001 — no regression in GitHub SBOM retrieval

#### Non-Functional Expectations

- Adding GitLab support must not increase the container image size by more than 5 MB
- GitLab SBOM retrieval must complete within the same 30-second expectation as GitHub for comparable repository sizes
