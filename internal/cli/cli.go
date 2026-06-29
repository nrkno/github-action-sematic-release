package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/nrkno/semrel/internal/conventional"
	"github.com/nrkno/semrel/internal/env"
	"github.com/nrkno/semrel/internal/git"
	"github.com/nrkno/semrel/internal/github"
	"github.com/nrkno/semrel/internal/notes"
	"github.com/nrkno/semrel/internal/output"
	"github.com/nrkno/semrel/internal/semver"
)

// Interface injection for testing
type GitClient interface {
	FindLatestAnnotatedTag() (*git.Tag, error)
	ListCommitsSinceTag(tag *git.Tag) ([]git.Commit, error)
	CreateAnnotatedTag(name, message string) (*git.Tag, error)
	PushTag(ctx context.Context, tagName string, auth git.BasicAuth) error
}

type GitHubClient interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.Release, error)
	CreateRelease(ctx context.Context, owner, repo string, opts github.CreateReleaseOptions) (*github.Release, error)
	ListPRsForCommit(ctx context.Context, owner, repo, sha string) ([]github.PR, error)
	SearchPRsForCommit(ctx context.Context, query string) ([]github.PR, error)
	PostPRComment(ctx context.Context, owner, repo string, prNumber int, body string) error
	FindPRComment(ctx context.Context, owner, repo string, prNumber int, marker string) (bool, error)
}

// Root returns the root cobra command
func Root(gitClient GitClient, githubClient GitHubClient, logger *slog.Logger) *cobra.Command {
	root := &cobra.Command{
		Use:           "semrel",
		Short:         "Semantic release automation",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(cmdLint(gitClient, logger))
	root.AddCommand(cmdRelease(gitClient, githubClient, logger))
	root.AddCommand(cmdNotify(githubClient, logger))
	root.AddCommand(cmdNotes(gitClient, githubClient, logger))

	return root
}

// cmdLint validates conventional commits in a range
func cmdLint(gitClient GitClient, logger *slog.Logger) *cobra.Command {
	var fromRef, toRef string

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Validate conventional commits",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load environment
			ghEnv := env.Load()

			// Load optional .semrelrc.yml from working directory.
			// Absent file → DefaultLintOptions (all rules at defaults).
			// Present file → overrides only the fields specified.
			wd := os.Getenv("INPUT_WORKING_DIRECTORY")
			if wd == "" {
				wd = "."
			}
			cfg, cfgErr := conventional.LoadConfig(filepath.Join(wd, ".semrelrc.yml"))
			if cfgErr != nil {
				logger.Error("failed to load .semrelrc.yml", "error", cfgErr)
				return cfgErr
			}
			lintOpts := conventional.DefaultLintOptions()
			if cfg != nil {
				lintOpts = conventional.LintOptions{
					CapitalFirstLetter: cfg.Lint.Rules.CapitalFirstLetter,
					RequireScope:       cfg.Lint.Rules.RequireScope,
				}
			}

			// Determine lint range based on context
			switch ghEnv.EventName {
			case "pull_request":
				// PR context: base → HEAD
				if fromRef == "" {
					fromRef = ghEnv.BaseRef
				}
				if toRef == "" {
					toRef = "HEAD"
				}
			case "push", "release":
				// Push/release context: previous tag → HEAD
				if fromRef == "" {
					tag, err := gitClient.FindLatestAnnotatedTag()
					if err != nil {
						logger.Error("failed to find latest tag", "error", err)
						return err
					}
					if tag != nil {
						fromRef = tag.Name
					}
				}
				if toRef == "" {
					toRef = "HEAD"
				}
			default:
				// Bootstrap: all commits
				if fromRef == "" {
					fromRef = ""
				}
				if toRef == "" {
					toRef = "HEAD"
				}
			}

			// List commits in range
			var commits []git.Commit
			var err error

			if fromRef == "" {
				// Bootstrap: all commits
				commits, err = gitClient.ListCommitsSinceTag(nil)
			} else {
				// Range: from tag to HEAD
				// For now, we'll use ListCommitsSinceTag and filter
				// This is a simplification; a full implementation would support arbitrary ranges
				var tag *git.Tag
				tag, err = gitClient.FindLatestAnnotatedTag()
				if err != nil {
					logger.Error("failed to find tag", "error", err)
					return err
				}
				commits, err = gitClient.ListCommitsSinceTag(tag)
			}

			if err != nil {
				logger.Error("failed to list commits", "error", err)
				return err
			}

			// Convert to RawCommit for validation
			rawCommits := make([]conventional.RawCommit, len(commits))
			for i, c := range commits {
				rawCommits[i] = conventional.RawCommit{
					SHA:     c.SHA,
					Message: c.Message,
				}
			}

			// Validate
			violations := conventional.ValidateAll(rawCommits, lintOpts)

			if len(violations) > 0 {
				// Output violations to stderr
				for _, v := range violations {
					fmt.Fprintf(os.Stderr, "commit %s: %s\n  %s\n  example: %s\n",
						v.ShortSHA, v.Rule, v.RawMessage, v.Example)
				}
				return fmt.Errorf("commit validation failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&fromRef, "from-ref", "", "commit range start")
	cmd.Flags().StringVar(&toRef, "to-ref", "", "commit range end")

	return cmd
}

// cmdRelease creates a release and tag
func cmdRelease(gitClient GitClient, githubClient GitHubClient, logger *slog.Logger) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "release",
		Short: "Create a release and tag",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load environment
			ghEnv := env.Load()

			// Parse repository
			parts := strings.Split(ghEnv.Repository, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid repository format: %s", ghEnv.Repository)
			}
			owner, repo := parts[0], parts[1]

			// Find latest tag
			latestTag, err := gitClient.FindLatestAnnotatedTag()
			if err != nil {
				logger.Error("failed to find latest tag", "error", err)
				return err
			}

			// List commits since tag
			commits, err := gitClient.ListCommitsSinceTag(latestTag)
			if err != nil {
				logger.Error("failed to list commits", "error", err)
				return err
			}

			// Parse commits
			var parsedCommits []conventional.Commit
			commitTypes := []string{}
			for _, c := range commits {
				parsed, err := conventional.Parse(conventional.RawCommit{
					SHA:     c.SHA,
					Message: c.Message,
				})
				if err != nil {
					logger.Warn("failed to parse commit", "sha", c.ShortSHA, "error", err)
					continue
				}
				parsed.SHA = c.SHA
				parsed.ShortSHA = c.ShortSHA
				parsedCommits = append(parsedCommits, parsed)
				commitTypes = append(commitTypes, string(parsed.Type))
			}

			// Detect bump type
			bumpType := semver.DetectBumpType(commitTypes)

			// Calculate next version
			var nextVersion semver.Version
			if latestTag == nil {
				// Bootstrap: start at 0.0.1
				nextVersion = semver.Version{Major: 0, Minor: 0, Patch: 1}
			} else {
				// Parse current version from tag
				currentVersion, err := semver.ParseVersion(latestTag.Name)
				if err != nil {
					logger.Error("failed to parse version", "error", err)
					return err
				}
				nextVersion = semver.NextVersion(currentVersion, bumpType)
			}

			versionTag := nextVersion.Tag()

			// Idempotency ladder
			// Rung 1: Check if release already exists
			existingRelease, err := githubClient.GetReleaseByTag(ctx, owner, repo, versionTag)
			if err == nil && existingRelease != nil {
				// Release exists, noop
				logger.Info("release already exists", "tag", versionTag)
				return outputReleaseFields(ghEnv.Output, nextVersion, true)
			}

			// Rung 2: Check if tag exists with matching SHA
			if latestTag != nil {
				// Get current HEAD SHA
				// For now, we'll assume we can get it from the environment or git
				// This is a simplification
				headCommits, err := gitClient.ListCommitsSinceTag(nil)
				if err == nil && len(headCommits) > 0 {
					headSHA := headCommits[0].SHA
					if latestTag.TargetSHA() == headSHA {
						// Tag exists and SHA matches, create release only
						if !dryRun {
							releaseNotes := generateReleaseNotes(parsedCommits, nil)
							_, err := githubClient.CreateRelease(ctx, owner, repo, github.CreateReleaseOptions{
								TagName: versionTag,
								Body:    releaseNotes,
							})
							if err != nil {
								logger.Error("failed to create release", "error", err)
								return err
							}
						}
						logger.Info("created release for existing tag", "tag", versionTag)
						return outputReleaseFields(ghEnv.Output, nextVersion, true)
					} else if latestTag.TargetSHA() != headSHA {
						// Tag exists but SHA mismatch, conflict
						return fmt.Errorf("tag %s exists but points to different commit: %s vs %s",
							latestTag.Name, latestTag.TargetSHA()[:7], headSHA[:7])
					}
				}
			}

			// Rung 3: Full flow - create tag, push, then create release.
			// Order is critical: PushTag MUST come before CreateRelease.
			// If CreateRelease fires first, GitHub auto-creates a lightweight
			// remote tag pointing at the commit. go-git then fails pushing the
			// local annotated tag object over it ("object not found").
			if !dryRun {
				releaseNotes := generateReleaseNotes(parsedCommits, nil)

				// 1. Create local annotated tag
				_, err := gitClient.CreateAnnotatedTag(versionTag, releaseNotes)
				if err != nil {
					logger.Error("failed to create tag", "error", err)
					return err
				}

				// 2. Push tag to remote BEFORE creating the release
				auth := git.BasicAuth{
					Username: "x-access-token",
					Password: ghEnv.Token,
				}
				err = gitClient.PushTag(ctx, versionTag, auth)
				if err != nil {
					logger.Error("failed to push tag", "error", err)
					return err
				}

				// 3. Create release — tag now exists on remote as annotated
				_, err = githubClient.CreateRelease(ctx, owner, repo, github.CreateReleaseOptions{
					TagName: versionTag,
					Body:    releaseNotes,
				})
				if err != nil {
					logger.Error("failed to create release", "error", err)
					return err
				}
			}

			logger.Info("release created", "tag", versionTag)
			return outputReleaseFields(ghEnv.Output, nextVersion, true)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "don't push tag or create release")

	return cmd
}

// cmdNotify posts a PR comment with release info
func cmdNotify(githubClient GitHubClient, logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Post PR comment with release info",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load environment
			ghEnv := env.Load()

			// Check if released flag is set to false
			released := os.Getenv("SEMREL_RELEASED")
			if released == "false" {
				logger.Info("skipping notify: SEMREL_RELEASED=false")
				return nil
			}

			// Only run on PR events
			if ghEnv.EventName != "pull_request" {
				logger.Info("skipping notify: not a pull request event")
				return nil
			}

			if ghEnv.PRNumber == 0 {
				return fmt.Errorf("PR number not found in environment")
			}

			// Parse repository
			parts := strings.Split(ghEnv.Repository, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid repository format: %s", ghEnv.Repository)
			}
			owner, repo := parts[0], parts[1]

			// Get version from environment
			version := os.Getenv("SEMREL_VERSION")
			if version == "" {
				version = "unknown"
			}

			// Check if comment already exists
			marker := fmt.Sprintf("<!-- semrel-notify:%s -->", version)
			found, err := githubClient.FindPRComment(ctx, owner, repo, ghEnv.PRNumber, marker)
			if err != nil {
				logger.Error("failed to check for existing comment", "error", err)
				return err
			}

			if found {
				logger.Info("comment already exists", "pr", ghEnv.PRNumber)
				return nil
			}

			// Post comment
			body := fmt.Sprintf("%s\n🎉 Release %s created!", marker, version)
			err = githubClient.PostPRComment(ctx, owner, repo, ghEnv.PRNumber, body)
			if err != nil {
				logger.Error("failed to post comment", "error", err)
				return err
			}

			logger.Info("comment posted", "pr", ghEnv.PRNumber)
			return nil
		},
	}

	return cmd
}

// cmdNotes generates release notes
func cmdNotes(gitClient GitClient, githubClient GitHubClient, logger *slog.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Generate release notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load environment
			ghEnv := env.Load()

			// Parse repository
			parts := strings.Split(ghEnv.Repository, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid repository format: %s", ghEnv.Repository)
			}
			owner, repo := parts[0], parts[1]

			// Find latest tag
			latestTag, err := gitClient.FindLatestAnnotatedTag()
			if err != nil {
				logger.Error("failed to find latest tag", "error", err)
				return err
			}

			// List commits since tag
			commits, err := gitClient.ListCommitsSinceTag(latestTag)
			if err != nil {
				logger.Error("failed to list commits", "error", err)
				return err
			}

			// Parse commits
			var parsedCommits []conventional.Commit
			for _, c := range commits {
				parsed, err := conventional.Parse(conventional.RawCommit{
					SHA:     c.SHA,
					Message: c.Message,
				})
				if err != nil {
					logger.Warn("failed to parse commit", "sha", c.ShortSHA, "error", err)
					continue
				}
				parsed.SHA = c.SHA
				parsed.ShortSHA = c.ShortSHA
				parsedCommits = append(parsedCommits, parsed)
			}

			// Build PR map
			prMap := make(map[string]notes.PR)
			for _, commit := range parsedCommits {
				prs, err := githubClient.ListPRsForCommit(ctx, owner, repo, commit.SHA)
				if err != nil {
					logger.Warn("failed to list PRs for commit", "sha", commit.ShortSHA, "error", err)
					continue
				}
				for _, pr := range prs {
					prMap[commit.SHA] = notes.PR{
						Number: pr.Number,
						URL:    pr.URL,
					}
				}
			}

			// Generate notes
			releaseNotes := notes.Generate(parsedCommits, prMap)

			// Output
			if _, err := fmt.Fprint(cmd.OutOrStdout(), releaseNotes.Body); err != nil {
				logger.Warn("failed to write notes", "error", err)
			}

			// Also write to GITHUB_OUTPUT if set
			if ghEnv.Output != "" {
				err := output.WriteFields(ghEnv.Output, map[string]string{
					"notes": releaseNotes.Body,
				})
				if err != nil {
					logger.Error("failed to write output", "error", err)
					return err
				}
			}

			return nil
		},
	}

	return cmd
}

// Helper functions

func generateReleaseNotes(commits []conventional.Commit, prMap map[string]notes.PR) string {
	if prMap == nil {
		prMap = make(map[string]notes.PR)
	}
	releaseNotes := notes.Generate(commits, prMap)
	return releaseNotes.Body
}

func outputReleaseFields(outputFile string, version semver.Version, released bool) error {
	fields := map[string]string{
		"version":  version.String(),
		"tag":      version.Tag(),
		"major":    fmt.Sprintf("%d", version.Major),
		"minor":    fmt.Sprintf("%d", version.Minor),
		"patch":    fmt.Sprintf("%d", version.Patch),
		"released": fmt.Sprintf("%v", released),
	}
	return output.WriteFields(outputFile, fields)
}
