# syntax=docker/dockerfile:1

# Stage 1: Build the static Go binary
FROM golang:1.26-alpine AS builder

ARG VERSION=dev

WORKDIR /src

# Copy module files first for layer caching
COPY go.mod ./

# Copy source
COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOFLAGS=-mod=mod go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /sbom-retriever \
    ./cmd/sbom-retriever

# Stage 2: Minimal runtime image
FROM gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.source="https://github.com/co-cddo/octo-observability-public-workflows"
LABEL org.opencontainers.image.description="Retrieves SBOMs from GitHub and submits them to the OCTO observability platform"
LABEL org.opencontainers.image.licenses="MIT"

COPY --from=builder /sbom-retriever /sbom-retriever

ENTRYPOINT ["/sbom-retriever"]
