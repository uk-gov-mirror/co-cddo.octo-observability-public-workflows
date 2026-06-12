package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/co-cddo/octo-observability-public-workflows/internal/config"
	ghclient "github.com/co-cddo/octo-observability-public-workflows/internal/github"
	"github.com/co-cddo/octo-observability-public-workflows/internal/ingestion"
	"github.com/co-cddo/octo-observability-public-workflows/internal/logger"
)

// version is injected at build time via -ldflags="-X main.version=..."
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Environ()))
}

func run(args []string, environ []string) int {
	cfg, err := config.Parse(args, environ)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	if cfg.Version {
		fmt.Println(version)
		return 0
	}

	// Initialise logger early — before validation so we can log validation errors.
	log := logger.New([]string{cfg.GitHubToken, cfg.APIKey})

	result := config.Validate(cfg)
	if !result.Valid {
		for _, e := range result.Errors {
			log.Error(e)
		}
		return 1
	}

	// Set up signal-aware context for graceful cancellation.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Split owner/repo.
	parts := strings.SplitN(cfg.Repo, "/", 2)
	owner, repo := parts[0], parts[1]

	log.Info("retrieving SBOM",
		logger.F("repo", cfg.Repo),
	)

	gh := ghclient.NewClient(cfg.GitHubToken, log)
	sbomBytes, err := gh.FetchSBOM(ctx, owner, repo, cfg.PollInterval, cfg.PollMaxAttempts)
	if err != nil {
		log.Error("failed to retrieve SBOM",
			logger.F("repo", cfg.Repo),
			logger.F("error", err.Error()),
		)
		return 1
	}

	log.Info("submitting SBOM",
		logger.F("repo", cfg.Repo),
		logger.F("service_id", cfg.ServiceID),
	)

	ingestClient := ingestion.NewClient(cfg.APIURL, cfg.APIKey, log)
	submission, err := ingestClient.Submit(ctx, cfg.ServiceID, sbomBytes)
	if err != nil {
		log.Error("failed to submit SBOM",
			logger.F("repo", cfg.Repo),
			logger.F("error", err.Error()),
		)
		return 1
	}

	if !submission.Success {
		log.Error("ingestion API rejected SBOM",
			logger.F("repo", cfg.Repo),
			logger.F("status", fmt.Sprintf("%d", submission.StatusCode)),
			logger.F("error", submission.ErrorMessage),
		)
		return 1
	}

	log.Info("SBOM submitted successfully",
		logger.F("repo", cfg.Repo),
		logger.F("service_id", cfg.ServiceID),
		logger.F("status", fmt.Sprintf("%d", submission.StatusCode)),
	)
	return 0
}
