package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/nrkno/semrel/internal/cli"
	"github.com/nrkno/semrel/internal/git"
	"github.com/nrkno/semrel/internal/github"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Setup logger (stderr)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create clients - open git repo lazily in each command
	ctx := context.Background()
	var gitClient cli.GitClient
	gitRepo, err := git.OpenRepo(".")
	if err != nil {
		// For help/version commands, we don't need the repo
		// So we'll create a lazy client and let the command handle it
		logger.Debug("git repo not available", "error", err)
		gitClient = &lazyGitClient{err: err}
	} else {
		gitClient = gitRepo
	}

	githubClient := github.NewClient(os.Getenv("GITHUB_TOKEN"), "")

	// Create and execute root command
	root := cli.Root(gitClient, githubClient, logger)
	root.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)

	if err := root.ExecuteContext(ctx); err != nil {
		logger.Error("command failed", "error", err)
		os.Exit(1)
	}
}

// lazyGitClient wraps a git client and defers errors until actual use
type lazyGitClient struct {
	err error
}

func (c *lazyGitClient) FindLatestAnnotatedTag(tagPrefix string) (*git.Tag, error) {
	return nil, c.err
}

func (c *lazyGitClient) FindTagByName(name string) (*git.Tag, error) {
	return nil, c.err
}

func (c *lazyGitClient) ListCommitsSinceTag(tag *git.Tag) ([]git.Commit, error) {
	return nil, c.err
}

func (c *lazyGitClient) CreateAnnotatedTag(name, message string) (*git.Tag, error) {
	return nil, c.err
}

func (c *lazyGitClient) PushTag(ctx context.Context, tagName string, auth git.BasicAuth) error {
	return c.err
}
