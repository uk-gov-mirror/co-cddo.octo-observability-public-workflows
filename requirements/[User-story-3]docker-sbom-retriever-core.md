# Story Decomposition: Multi-Platform Docker SBOM Retriever

## INVEST Analysis

### Abstract Task: "Multi-Platform Docker SBOM Retriever"

**Analysis Dimensions**:
- **Core Responsibility**: Extract the SBOM retrieval logic into a standalone Docker image that can run in any CI system, authenticate via API key or OIDC, and retrieve SBOMs from multiple source control platforms
- **Primary Operations**: Authenticate with source control platform API, retrieve SBOM, authenticate with ingestion API (API key or OIDC token exchange), submit SBOM to ingestion endpoint
- **Key Constraints**: Must support multiple source control platforms (GitHub, GitLab, etc.), multiple auth mechanisms (API key, OIDC), multiple CI systems (GitHub Actions, GitLab CI, Jenkins, etc.), backwards-compatible GitHub Action wrapper
- **Technical Complexity**: High — multi-platform abstraction, OIDC token exchange, Docker multi-arch builds, CI-agnostic design
- **Business Complexity**: Medium — same core data flow as before (retrieve SBOM → submit), but now with platform abstraction and multiple auth strategies

### INVEST Evaluation
- ✅ **Independent**: Can be developed as a new container alongside the existing action
- ✅ **Negotiable**: Platform priority, auth mechanism details, CLI interface design are negotiable
- ✅ **Valuable**: Unlocks non-GitHub customers and non-GitHub-Actions CI systems
- ✅ **Estimable**: Clear scope once split into platform-specific stories
- ❌ **Small**: Too large as a single story — needs splitting
- ✅ **Testable**: Each platform and auth mechanism has clear test scenarios

**Conclusion**: Needs splitting

### Split Strategy

**By capability layer + platform extension**:
1. **Core Docker image with GitHub SBOM retrieval and API key auth** — baseline functionality equivalent to existing action, containerised
2. **OIDC token exchange authentication** — adds federated identity auth as an alternative to API keys
3. **GitLab SBOM retrieval** — extends the retriever to support GitLab's dependency API
4. **Backwards-compatible GitHub Action wrapper** — wraps the Docker image for existing GitHub Action consumers

**Rationale**: Each story delivers independent value. Story 1 produces a working `docker run` for GitHub users in any CI. Stories 2-4 extend it without breaking existing functionality.

---

## [STORY-002-001] Docker SBOM Retriever with GitHub Support and API Key Auth

### Background

The current SBOM submission capability is tightly coupled to GitHub Actions as both the CI runtime and the source control platform. Organisations using Jenkins, GitLab CI, or other CI systems cannot use the existing action. By extracting the retrieval and submission logic into a standalone Docker image invoked via `docker run`, any CI system that can run containers gains the ability to submit SBOMs. This first story delivers the core image with GitHub SBOM retrieval and API key authentication — functionally equivalent to the existing action but portable.

### Business Value

- Enable any CI system (Jenkins, GitLab CI, CircleCI, etc.) to submit GitHub-hosted SBOMs to the platform via a simple `docker run`
- Remove GitHub Actions as a hard dependency for customers who use GitHub for source control but a different CI system
- Establish the containerised foundation that subsequent stories extend with additional platforms and auth mechanisms

### Dependencies and Assumptions

- **Prerequisites**: The existing SBOM ingestion API endpoint accepts submissions via POST with API key authentication (already in production per STORY-001-001)
- **Data assumptions**: Target repositories have GitHub's dependency graph enabled; the GitHub API token provided has sufficient permissions to read the dependency graph
- **Integration points**: GitHub Dependency Graph API (read), SBOM ingestion API (write)
- **Business constraints**: Image must be publicly available (e.g., GitHub Container Registry) for third-party consumption; image must support linux/amd64 and linux/arm64 architectures

### Scope In

- Dockerfile producing a minimal, multi-arch container image (linux/amd64, linux/arm64)
- CLI entrypoint accepting configuration via environment variables and/or command-line flags
- GitHub SBOM retrieval using a provided GitHub token
- SBOM submission to a configurable ingestion API endpoint using API key authentication
- Input validation with clear error messages for missing or invalid configuration
- Non-zero exit code on failure (enables CI systems to fail the pipeline)
- Structured logging (stdout) for observability within CI logs
- Published to GitHub Container Registry (ghcr.io)

### Scope Out

- OIDC/token exchange authentication (STORY-002-002)
- GitLab or other non-GitHub source control platforms (STORY-002-003)
- Backwards-compatible GitHub Action wrapper (STORY-002-004)
- Retry logic with exponential backoff (enhancement for a future story)
- Caching of SBOMs between runs
- Any UI or dashboard features

### Acceptance Criteria

#### AC1: Successful SBOM retrieval and submission from Docker

**Given** a GitHub repository "org/repo" with dependency graph enabled, a valid GitHub token with `contents:read` permission, a valid ingestion API key, and the ingestion API endpoint URL
**When** a user runs `docker run ghcr.io/co-cddo/sbom-retriever --repo org/repo --github-token <token> --api-key <key> --api-url https://ingest.example.com`
**Then** the container retrieves the SBOM from GitHub's dependency graph API and submits it to the ingestion endpoint, exits with code 0, and logs a success message including the repository name

#### AC2: Multi-architecture image availability

**Given** a user on a linux/arm64 CI runner (e.g., AWS Graviton, Apple Silicon)
**When** they pull the image from ghcr.io
**Then** the correct architecture variant is pulled and the container executes successfully without emulation

#### AC3: Missing required configuration

**Given** a user runs the container without providing the `--repo` parameter
**When** the container starts
**Then** it exits with a non-zero exit code within 5 seconds and logs a clear error message indicating which required parameter is missing

#### AC4: Invalid GitHub token

**Given** a user provides an expired or invalid GitHub token
**When** the container attempts to retrieve the SBOM from GitHub
**Then** it exits with a non-zero exit code and logs an error message indicating authentication with GitHub failed, without exposing the token value in the logs

#### AC5: Ingestion API rejects the submission

**Given** the ingestion API returns an HTTP 401 (invalid API key) or HTTP 422 (invalid payload)
**When** the container receives the error response
**Then** it exits with a non-zero exit code and logs the HTTP status and error message from the API, enabling the user to diagnose the issue from CI logs alone

#### AC6: Repository without dependency graph enabled

**Given** a GitHub repository that does not have the dependency graph feature enabled
**When** the container attempts to retrieve the SBOM
**Then** it exits with a non-zero exit code and logs a clear message indicating the dependency graph is not available for that repository, with guidance to enable it

#### Non-Functional Expectations

- Container image size must be under 50 MB (compressed) to ensure fast pulls in CI environments
- Container must complete a successful retrieval and submission within 30 seconds for a typical repository (under 500 dependencies)
