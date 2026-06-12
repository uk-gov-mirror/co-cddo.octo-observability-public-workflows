# Story: OIDC Token Exchange Authentication

## [STORY-002-002] OIDC Token Exchange Authentication for Ingestion API

### Background

API keys are simple but have operational downsides: they must be rotated manually, can be leaked in logs or config files, and grant long-lived access. Many CI systems (GitHub Actions, GitLab CI, AWS CodeBuild) support OpenID Connect (OIDC) identity tokens that allow workloads to prove their identity without storing long-lived secrets. By adding OIDC token exchange as an authentication option, the SBOM retriever can authenticate with the ingestion API using short-lived, automatically-issued tokens — reducing secret management burden and improving security posture.

### Business Value

- Eliminate the need for customers to create, store, and rotate long-lived API keys for SBOM submission
- Enable zero-secret CI pipelines for customers whose CI platforms support OIDC (GitHub Actions, GitLab CI 16+, AWS CodeBuild)
- Improve security posture by using short-lived tokens scoped to a single CI run
- Align with enterprise security requirements that mandate federated identity over static credentials

### Dependencies and Assumptions

- **Prerequisites**: STORY-002-001 (core Docker image with API key auth) is complete and functional
- **Data assumptions**: The ingestion API's token exchange endpoint exists and accepts OIDC tokens from configured identity providers; the CI platform issues OIDC tokens to the running workload
- **Integration points**: CI platform's OIDC token provider (read), ingestion API token exchange endpoint (write), ingestion API submission endpoint (write with exchanged token)
- **Business constraints**: Must not break existing API key authentication — OIDC is an alternative, not a replacement

### Scope In

- New auth mode flag (e.g., `--auth-mode oidc`) alongside existing API key mode
- Retrieve OIDC token from the CI environment (environment variable or metadata endpoint, configurable)
- Exchange OIDC token with the ingestion API's token exchange endpoint for a short-lived access token
- Use the exchanged token for SBOM submission
- Clear error messaging when OIDC token is not available in the environment
- Clear error messaging when token exchange fails (e.g., identity provider not configured on the server side)
- Documentation of which CI platforms are supported and how to configure OIDC

### Scope Out

- Implementing the token exchange endpoint on the ingestion API server side (separate service responsibility)
- Custom OIDC provider configuration (only standard CI platform providers supported initially)
- Token caching across multiple runs
- Mutual TLS or other non-OIDC federated auth mechanisms
- API key deprecation or removal

### Acceptance Criteria

#### AC1: Successful SBOM submission using OIDC from GitHub Actions

**Given** a GitHub Actions workflow with `id-token: write` permission configured, the OIDC audience set to the ingestion API's expected audience, and the ingestion API configured to trust GitHub's OIDC issuer
**When** a user runs the container with `--auth-mode oidc --oidc-token-env ACTIONS_ID_TOKEN_REQUEST_TOKEN` (or equivalent)
**Then** the container obtains the OIDC token from the environment, exchanges it with the ingestion API for an access token, submits the SBOM successfully, and exits with code 0

#### AC2: Successful SBOM submission using OIDC from GitLab CI

**Given** a GitLab CI job with `id_tokens` configured for the ingestion API audience, and the ingestion API configured to trust GitLab's OIDC issuer
**When** a user runs the container with `--auth-mode oidc --oidc-token-env CI_JOB_JWT_V2` (or equivalent)
**Then** the container obtains the OIDC token from the environment, exchanges it for an access token, submits the SBOM, and exits with code 0

#### AC3: OIDC token not available in environment

**Given** a CI environment that does not provide an OIDC token (e.g., Jenkins without OIDC plugin)
**When** a user runs the container with `--auth-mode oidc`
**Then** the container exits with a non-zero exit code within 5 seconds and logs an error message explaining that no OIDC token was found in the expected environment variable, with guidance on configuring the CI platform for OIDC

#### AC4: Token exchange rejected by ingestion API

**Given** the ingestion API does not recognise the CI platform's OIDC issuer (e.g., the trust relationship has not been configured)
**When** the container attempts to exchange the OIDC token
**Then** it exits with a non-zero exit code and logs the error response from the token exchange endpoint, including guidance to verify the issuer trust configuration on the ingestion API

#### AC5: API key auth continues to work unchanged

**Given** a user with an existing API key who does not use OIDC
**When** they run the container with `--auth-mode apikey --api-key <key>` (or without specifying auth-mode, defaulting to API key)
**Then** the behaviour is identical to STORY-002-001 — no regression in API key authentication flow

#### Non-Functional Expectations

- The OIDC token exchange adds no more than 2 seconds to the total execution time compared to direct API key authentication
- OIDC tokens and exchanged access tokens must never appear in container logs at any log level
