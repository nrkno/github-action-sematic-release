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
	FindTagByName(name string) (*git.Tag, error)
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

			// Log 1: commits in release
			logger.Info("commits in release",
				"count", len(parsedCommits),
				"feat", countByType(parsedCommits, conventional.TypeFeat),
				"fix", countByType(parsedCommits, conventional.TypeFix),
				"breaking", countBreaking(parsedCommits),
			)

			// Detect bump type
			bump := semver.DetectBumpType(commitTypes)

			// Calculate next version
			var currentVersion semver.Version
			var nextVersion semver.Version
			if latestTag == nil {
				// Bootstrap: start at 0.0.1
				nextVersion = semver.Version{Major: 0, Minor: 0, Patch: 1}
				// Log 8: bootstrap case
				logger.Info("no prior annotated tags found — bootstrapping version",
					"version", nextVersion.Tag(),
				)
			} else {
				// Parse current version from tag
				currentVersion, err = semver.ParseVersion(latestTag.Name)
				if err != nil {
					logger.Error("failed to parse version", "error", err)
					return err
				}
				nextVersion = semver.NextVersion(currentVersion, bump)
			}

			// Log 2: bump detected
			logger.Info("bump detected",
				"type", bump.String(),
				"from", currentVersion.Tag(),
				"to", nextVersion.Tag(),
			)

			versionTag := nextVersion.Tag()

			// Fetch PRs for all commits in this release (used in logs 3/4 and release notes)
			prMap := fetchPRsForCommits(ctx, githubClient, owner, repo, parsedCommits, logger)

			// Log 3: PRs included in the release
			if len(prMap) > 0 {
				for sha, pr := range prMap {
					logger.Info("PR in release",
						"pr", fmt.Sprintf("#%d", pr.Number),
						"title", pr.Title,
						"sha", sha[:7],
					)
				}
			}

			// Log 4: release triggered by PR or commit
			if triggerCommit := findTriggerCommit(parsedCommits, bump); triggerCommit != nil {
				if pr, ok := prMap[triggerCommit.SHA]; ok {
					logger.Info("release triggered by PR",
						"pr", fmt.Sprintf("#%d", pr.Number),
						"title", pr.Title,
						"url", pr.URL,
					)
			} else {
				logger.Info("release triggered by commit",
					"sha", triggerCommit.ShortSHA,
					"message", commitSubject(triggerCommit.RawMessage),
				)
			}
			}

			// Convert github.PR map to notes.PR map for release notes generation
			notesPRMap := githubPRMapToNotesPRMap(prMap)

			// Idempotency ladder
			// Rung 1: Check if release already exists
			existingRelease, err := githubClient.GetReleaseByTag(ctx, owner, repo, versionTag)
			if err == nil && existingRelease != nil {
				// Release exists, noop
				logger.Info("release already exists", "tag", versionTag)
				return outputReleaseFields(ghEnv.Output, nextVersion, false)
			}

			// Rung 2: Check whether the computed next-version tag already exists.
			// This handles the retry case: tag was pushed in a previous interrupted
			// run, but the GitHub release was not yet created.
			existingVersionTag, err := gitClient.FindTagByName(versionTag)
			if err != nil {
				logger.Error("failed to look up version tag", "tag", versionTag, "error", err)
				return err
			}
			if existingVersionTag != nil {
				// The next-version tag already exists — check it points to HEAD
				headCommits, err := gitClient.ListCommitsSinceTag(nil)
				if err != nil || len(headCommits) == 0 {
					return fmt.Errorf("could not determine HEAD SHA")
				}
				headSHA := headCommits[0].SHA
				if existingVersionTag.TargetSHA() != headSHA {
					// Tag points to a different commit — genuine conflict
					return fmt.Errorf("tag %s exists but points to different commit: %s vs %s",
						versionTag, existingVersionTag.TargetSHA()[:7], headSHA[:7])
				}
				// Tag already at HEAD — just create the GitHub release (idempotent retry)
				if !dryRun {
					releaseNotes := generateReleaseNotes(parsedCommits, notesPRMap)
					release, err := githubClient.CreateRelease(ctx, owner, repo, github.CreateReleaseOptions{
						TagName: versionTag,
						Body:    releaseNotes,
					})
					if err != nil {
						logger.Error("failed to create release for existing tag", "error", err)
						return err
					}
					// Log 7: GitHub release created
					logger.Info("created GitHub release",
						"tag", release.TagName,
						"url", release.HTMLURL,
					)
				}
				logger.Info("created release for existing tag", "tag", versionTag)
				return outputReleaseFields(ghEnv.Output, nextVersion, true)
			}
			// Rung 3 (full flow) falls through here

			// Rung 3: Full flow - create tag, push, then create release.
			// Order is critical: PushTag MUST come before CreateRelease.
			// If CreateRelease fires first, GitHub auto-creates a lightweight
			// remote tag pointing at the commit. go-git then fails pushing the
			// local annotated tag object over it ("object not found").
			if !dryRun {
				releaseNotes := generateReleaseNotes(parsedCommits, notesPRMap)

				// 1. Create local annotated tag
				tag, err := gitClient.CreateAnnotatedTag(versionTag, releaseNotes)
				if err != nil {
					logger.Error("failed to create tag", "error", err)
					return err
				}
				// Log 5: annotated tag created
				logger.Info("created annotated tag",
					"tag", tag.Name,
					"commit", tag.TargetSHA()[:7],
				)

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
				// Log 6: tag pushed
				logger.Info("pushed tag to origin", "tag", versionTag)

				// 3. Create release — tag now exists on remote as annotated
				release, err := githubClient.CreateRelease(ctx, owner, repo, github.CreateReleaseOptions{
					TagName: versionTag,
					Body:    releaseNotes,
				})
				if err != nil {
					logger.Error("failed to create release", "error", err)
					return err
				}
				// Log 7: GitHub release created
				logger.Info("created GitHub release",
					"tag", release.TagName,
					"url", release.HTMLURL,
				)
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

func countByType(commits []conventional.Commit, t conventional.CommitType) int {
	n := 0
	for _, c := range commits {
		if c.Type == t {
			n++
		}
	}
	return n
}

func countBreaking(commits []conventional.Commit) int {
	n := 0
	for _, c := range commits {
		if c.Breaking {
			n++
		}
	}
	return n
}

func fetchPRsForCommits(ctx context.Context, gh GitHubClient, owner, repo string, commits []conventional.Commit, logger *slog.Logger) map[string]github.PR {
	prMap := make(map[string]github.PR)
	for _, commit := range commits {
		prs, err := gh.ListPRsForCommit(ctx, owner, repo, commit.SHA)
		if err != nil {
			logger.Warn("failed to list PRs for commit", "sha", commit.ShortSHA, "error", err)
			continue
		}
		if len(prs) > 0 {
			prMap[commit.SHA] = prs[0]
		}
	}
	return prMap
}

func findTriggerCommit(commits []conventional.Commit, bump semver.BumpType) *conventional.Commit {
	for i := range commits {
		switch bump {
		case semver.BumpMajor:
			if commits[i].Breaking {
				return &commits[i]
			}
		case semver.BumpMinor:
			if commits[i].Type == conventional.TypeFeat {
				return &commits[i]
			}
		case semver.BumpPatch:
			if commits[i].Type == conventional.TypeFix {
				return &commits[i]
			}
		}
	}
	return nil
}

func githubPRMapToNotesPRMap(m map[string]github.PR) map[string]notes.PR {
	out := make(map[string]notes.PR, len(m))
	for k, pr := range m {
		out[k] = notes.PR{Number: pr.Number, URL: pr.URL}
	}
	return out
}

func generateReleaseNotes(commits []conventional.Commit, prMap map[string]notes.PR) string {
	if prMap == nil {
		prMap = make(map[string]notes.PR)
	}
	releaseNotes := notes.Generate(commits, prMap)
	return releaseNotes.Body
}

// commitSubject returns the first line of a commit message (the subject).
func commitSubject(message string) string {
	if i := strings.Index(message, "\n"); i != -1 {
		return message[:i]
	}
	return message
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
