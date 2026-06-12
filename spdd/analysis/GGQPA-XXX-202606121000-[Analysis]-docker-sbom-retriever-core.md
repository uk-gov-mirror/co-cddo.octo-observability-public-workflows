# SPDD Analysis: Docker SBOM Retriever with GitHub Support and API Key Auth

## Original Business Requirement

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

---

## Domain Concept Identification

### Existing Concepts (from codebase)

- **SBOM Retrieval (GitHub)**: The process of requesting an SBOM report from GitHub's dependency graph API (`generate-report` endpoint) and polling for completion (`fetch-report` endpoint). Already implemented in `src/github-api.ts` with async polling, timeout handling, and HTTP error mapping.
- **SBOM Submission**: The process of POSTing an SBOM payload to the ingestion API with API key authentication via `X-API-Key` header. Already implemented in `src/ingestion-api.ts` targeting the `/api/modules/sbom/services/{serviceId}` endpoint with `application/spdx+json` content type.
- **Input Validation**: Structured validation of user-provided configuration (URL format, UUID format, token presence). Already implemented in `src/validate.ts` with an errors-array pattern returning a `ValidationResult`.
- **Action Inputs**: The configuration surface — currently tightly coupled to GitHub Actions' `core.getInput()`. Comprises: `base-url`, `service-id`, `api-key`, `github-token`.

### New Concepts Required

- **CLI Entrypoint**: A new runtime entry point that replaces `@actions/core.getInput()` with environment variables and/or command-line flags, while reusing the same downstream logic. Bridges the containerised runtime to existing retrieval/submission logic.
- **Platform Abstraction (future-facing)**: While this story only implements GitHub, the architecture should allow future stories to add new source control platforms without restructuring. This is a design-time consideration, not a runtime concept for this story.
- **Structured Logging**: Replacement for `@actions/core.info()` and `@actions/core.setFailed()` — stdout-based logging suitable for any CI system, not just GitHub Actions' annotation system.
- **Container Distribution**: Multi-arch Docker image published to ghcr.io, with CI workflow for automated builds.

### Key Business Rules

- **Token secrecy**: Tokens and API keys must never appear in logs — currently enforced via `core.setSecret()` in the action; must be enforced via log sanitisation in the container.
- **Non-zero exit on failure**: CI systems rely on exit codes to determine pipeline success/failure — the container must exit non-zero for any retrieval or submission failure.
- **SBOM format passthrough**: The SBOM is retrieved from GitHub and submitted as-is to the ingestion API — no transformation or enrichment is performed.
- **Async SBOM generation**: GitHub's SBOM API is asynchronous (generate → poll → fetch). The retriever must handle the polling lifecycle with timeout protection.

---

## Strategic Approach

### Solution Direction

The core business logic (SBOM retrieval from GitHub, SBOM submission to ingestion API) already exists in well-tested, modular TypeScript. The strategic approach is to **decouple this logic from the GitHub Actions runtime** by introducing a thin CLI layer that reads configuration from environment variables/flags and delegates to the same retrieval and submission functions — then package the result in a minimal Docker image.

The existing `@actions/github` (Octokit) dependency for GitHub API calls is the only GitHub Actions SDK component used for business logic — the REST API calls it makes are standard GitHub API calls that can be made with any HTTP client. The `@actions/core` dependency is used purely for input/output/logging, making it a straightforward replacement target.

### Key Design Decisions

- **Rewrite vs. adapt existing code**: The existing code uses `@actions/github` (Octokit wrapper) and `@actions/http-client` — both pull in the Actions SDK. The Docker image should use standalone HTTP clients (native `fetch` or a lightweight library) to avoid bundling the Actions SDK. This means the retrieval and submission logic will be **reimplemented** rather than directly imported, but following the same patterns and test cases. → **Recommended: Clean reimplementation** — the existing code is small (~130 lines of business logic) and tightly coupled to Actions SDK types.

- **Language choice for the Docker image**: TypeScript/Node.js (matching existing codebase) vs. Go (smaller binary, no runtime dependency, natural for CLI tools and Docker images). Node.js keeps team familiarity and allows sharing test fixtures; Go produces a ~5MB static binary ideal for small container images. → **Trade-off**: Node.js for faster delivery and team consistency vs. Go for smaller image and better CLI ergonomics. Either is viable; the team's primary language expertise should drive this.

- **Configuration interface**: Environment variables only (simpler, standard for containers) vs. CLI flags (more discoverable, self-documenting with `--help`) vs. both (most flexible but more code). → **Recommended: Both, with env vars as primary** — CI systems naturally pass env vars, but flags improve developer ergonomics for local testing and debugging.

- **Multi-arch build strategy**: Docker buildx with QEMU emulation (simpler CI, slower builds) vs. native runners per architecture (faster builds, more CI complexity). → **Recommended: Docker buildx with QEMU** — the build is not performance-critical and runs infrequently (on release only).

### Alternatives Considered

- **Thin shell wrapper around the existing bundled JS**: Run `node dist/index.js` inside Docker with env-var-to-input shimming. Rejected because it still bundles the Actions SDK, produces a larger image, and the `core.setFailed()` / `core.getInput()` semantics don't map cleanly to a container runtime.
- **Keep as GitHub Action only, document `docker run` of the action image**: GitHub's Docker container action type already produces an image. Rejected because it's tied to the Actions runtime contract (entrypoint expectations, output format) and doesn't support non-GitHub-Actions CI.

---

## Risk & Gap Analysis

### Requirement Ambiguities

- **`service-id` parameter**: The existing action requires a `service-id` (UUID) but the story's AC1 example CLI omits it — only showing `--repo`, `--github-token`, `--api-key`, `--api-url`. The ingestion API endpoint is `/api/modules/sbom/services/{serviceId}`, so `service-id` is still required. The CLI interface in the ACs is illustrative, not prescriptive.
- **Logging format**: "Structured logging" is mentioned in Scope In but the format (JSON lines, human-readable key=value, plain text) is unspecified. Any CI-friendly format satisfies the requirement.
- **Image registry path**: AC1 uses `ghcr.io/co-cddo/sbom-retriever` — this implies a new repository or package name. The current repo is `octo-observability-public-workflows`. Whether the Docker image lives in the same repo (as a package) or a new repo needs clarification but is not a blocker for development.

### Edge Cases

- **SBOM generation timeout**: The existing code polls up to 24 times at 5-second intervals (2 minutes). In a container context, the user may have a shorter CI timeout — the container should respect both its internal timeout and external signals (SIGTERM from CI).
- **Large SBOM payloads**: Very large repositories could produce multi-MB SBOMs. The ingestion API may have payload size limits not documented in the story.
- **Network interruption during polling**: The existing implementation does not retry individual poll requests that fail transiently — a network blip during the polling phase would fail the entire run.
- **GitHub API rate limiting**: If the token is shared across multiple concurrent runs, rate limiting could cause failures. The error message should distinguish rate limiting from auth failures.

### Technical Risks

- **Image size constraint (50 MB compressed)**: If using Node.js, the base image (`node:22-alpine`) is ~50 MB alone. Achieving <50 MB requires either a distroless/scratch approach with bundled JS, or choosing Go for a ~5 MB static binary. This constraint materially influences the language choice.
- **Async polling in container context**: The 2-minute polling window is fine for a GitHub Action (which has generous timeouts) but may surprise users in CI systems with shorter step timeouts. The container should document its maximum execution time.
- **GitHub API endpoint for SBOM**: The `generate-report` and `fetch-report` endpoints used by the existing code appear to be undocumented/preview GitHub API endpoints. Their stability and availability outside of GitHub Actions context needs verification.

### Acceptance Criteria Coverage

| AC# | Description | Addressable? | Gaps/Notes |
|-----|-------------|--------------|------------|
| AC1 | Successful retrieval and submission | Yes | CLI example omits `service-id`; implementation must include it |
| AC2 | Multi-architecture image | Yes | Straightforward with Docker buildx |
| AC3 | Missing required config | Yes | Validation pattern already exists in codebase |
| AC4 | Invalid GitHub token | Yes | Error handling pattern already exists; must ensure token not logged |
| AC5 | Ingestion API rejection | Yes | Error handling already exists; sanitisation pattern in place |
| AC6 | Dependency graph not enabled | Yes | 404 handling already maps to this error message |

All ACs are fully addressable with the proposed approach. The only gap is the omission of `service-id` from the AC1 example command, which is a documentation issue in the story, not a technical blocker.
