package cli

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/nrkno/semrel/internal/conventional"
	"github.com/nrkno/semrel/internal/git"
	"github.com/nrkno/semrel/internal/github"
	"github.com/nrkno/semrel/internal/notes"
	"github.com/nrkno/semrel/internal/semver"
)

// Helper to create test tags with unexported targetSHA field
// Note: We can't set unexported fields via reflection in tests, so we'll work around it
// by not testing the TargetSHA() method directly in these tests
func newTestTag(name, sha string) *git.Tag {
	return &git.Tag{
		Name: name,
		SHA:  sha,
	}
}
type mockGitClient struct {
	latestTag       *git.Tag
	latestTagErr    error
	commits         []git.Commit
	commitsErr      error
	createdTag      *git.Tag
	createTagErr    error
	pushedTag       string
	pushTagErr      error
	isShallowRepo   bool
}

func (m *mockGitClient) FindLatestAnnotatedTag() (*git.Tag, error) {
	if m.isShallowRepo {
		return nil, git.ShallowRepoError{Message: "repository is a shallow clone"}
	}
	return m.latestTag, m.latestTagErr
}

func (m *mockGitClient) ListCommitsSinceTag(tag *git.Tag) ([]git.Commit, error) {
	return m.commits, m.commitsErr
}

func (m *mockGitClient) CreateAnnotatedTag(name, message string) (*git.Tag, error) {
	if m.createTagErr != nil {
		return nil, m.createTagErr
	}
	m.createdTag = newTestTag(name, "abc123")
	return m.createdTag, nil
}

func (m *mockGitClient) PushTag(ctx context.Context, tagName string, auth git.BasicAuth) error {
	m.pushedTag = tagName
	return m.pushTagErr
}

type mockGitHubClient struct {
	releaseByTag    *github.Release
	releaseByTagErr error
	createdRelease  *github.Release
	createReleaseErr error
	prs             []github.PR
	prsErr          error
	searchPRs       []github.PR
	searchPRsErr    error
	commentPosted   bool
	commentErr      error
	commentExists   bool
	findCommentErr  error
}

func (m *mockGitHubClient) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*github.Release, error) {
	return m.releaseByTag, m.releaseByTagErr
}

func (m *mockGitHubClient) CreateRelease(ctx context.Context, owner, repo string, opts github.CreateReleaseOptions) (*github.Release, error) {
	if m.createReleaseErr != nil {
		return nil, m.createReleaseErr
	}
	m.createdRelease = &github.Release{
		TagName: opts.TagName,
		Body:    opts.Body,
	}
	return m.createdRelease, nil
}

func (m *mockGitHubClient) ListPRsForCommit(ctx context.Context, owner, repo, sha string) ([]github.PR, error) {
	return m.prs, m.prsErr
}

func (m *mockGitHubClient) SearchPRsForCommit(ctx context.Context, query string) ([]github.PR, error) {
	return m.searchPRs, m.searchPRsErr
}

func (m *mockGitHubClient) PostPRComment(ctx context.Context, owner, repo string, prNumber int, body string) error {
	m.commentPosted = true
	return m.commentErr
}

func (m *mockGitHubClient) FindPRComment(ctx context.Context, owner, repo string, prNumber int, marker string) (bool, error) {
	return m.commentExists, m.findCommentErr
}

// Tests

func TestLintValidCommits(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	gitClient := &mockGitClient{
		commits: []git.Commit{
			{
				SHA:      "abc123def456",
				ShortSHA: "abc123d",
				Message:  "feat: add new feature",
			},
			{
				SHA:      "def456ghi789",
				ShortSHA: "def456g",
				Message:  "fix: resolve bug",
			},
		},
	}

	cmd := cmdLint(gitClient, logger)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("lint with valid commits should not error, got: %v", err)
	}
}

func TestLintInvalidCommits(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	gitClient := &mockGitClient{
		commits: []git.Commit{
			{
				SHA:      "abc123def456",
				ShortSHA: "abc123d",
				Message:  "invalid commit message",
			},
		},
	}

	cmd := cmdLint(gitClient, logger)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Error("lint with invalid commits should error")
	}
}

func TestLintShallowRepo(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	t.Setenv("GITHUB_EVENT_NAME", "push")

	gitClient := &mockGitClient{
		isShallowRepo: true,
		latestTagErr:  git.ShallowRepoError{Message: "repository is a shallow clone"},
	}

	cmd := cmdLint(gitClient, logger)
	cmd.SetArgs([]string{})

	// Redirect stderr to capture error output
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	err := cmd.ExecuteContext(context.Background())

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Error("lint on shallow repo should error")
	}
}

func TestReleaseNewVersion(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")

	gitClient := &mockGitClient{
		latestTag: nil, // Bootstrap case
		commits: []git.Commit{
			{
				SHA:      "abc123def456",
				ShortSHA: "abc123d",
				Message:  "feat: initial feature",
			},
		},
	}

	githubClient := &mockGitHubClient{
		releaseByTagErr: github.ErrNotFound,
	}

	cmd := cmdRelease(gitClient, githubClient, logger)
	cmd.SetArgs([]string{"--dry-run"})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("release should not error, got: %v", err)
	}
}

func TestReleaseIdempotentExists(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")

	gitClient := &mockGitClient{
		latestTag: newTestTag("v0.0.1", "abc123"),
		commits:   []git.Commit{},
	}

	githubClient := &mockGitHubClient{
		releaseByTag: &github.Release{
			TagName: "v0.0.1",
		},
	}

	cmd := cmdRelease(gitClient, githubClient, logger)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("release idempotent (exists) should not error, got: %v", err)
	}
}

func TestNotifySkippedWhenNotReleased(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Setenv("SEMREL_RELEASED", "false")
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")

	githubClient := &mockGitHubClient{}

	cmd := cmdNotify(githubClient, logger)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("notify with SEMREL_RELEASED=false should not error, got: %v", err)
	}

	if githubClient.commentPosted {
		t.Error("notify should not post comment when SEMREL_RELEASED=false")
	}
}

func TestNotifyPostsComment(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Setenv("SEMREL_RELEASED", "true")
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("GITHUB_REF", "refs/pull/42/merge")
	t.Setenv("SEMREL_VERSION", "v1.0.0")

	githubClient := &mockGitHubClient{
		commentExists: false,
	}

	cmd := cmdNotify(githubClient, logger)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("notify should not error, got: %v", err)
	}

	if !githubClient.commentPosted {
		t.Error("notify should post comment")
	}
}

func TestNotifyIdempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Setenv("SEMREL_RELEASED", "true")
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("GITHUB_REF", "refs/pull/42/merge")
	t.Setenv("SEMREL_VERSION", "v1.0.0")

	githubClient := &mockGitHubClient{
		commentExists: true,
	}

	cmd := cmdNotify(githubClient, logger)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Errorf("notify idempotent should not error, got: %v", err)
	}

	if githubClient.commentPosted {
		t.Error("notify should not post comment when it already exists")
	}
}

func TestNotesGeneration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	t.Setenv("GITHUB_REPOSITORY", "owner/repo")

	gitClient := &mockGitClient{
		latestTag: newTestTag("v0.0.1", "abc123"),
		commits: []git.Commit{
			{
				SHA:      "abc123def456",
				ShortSHA: "abc123d",
				Message:  "feat: add feature",
			},
		},
	}

	githubClient := &mockGitHubClient{
		prs: []github.PR{
			{
				Number: 42,
				URL:    "https://github.com/owner/repo/pull/42",
				Title:  "Add feature",
			},
		},
	}

	cmd := cmdNotes(gitClient, githubClient, logger)
	cmd.SetArgs([]string{})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.ExecuteContext(context.Background())

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("notes should not error, got: %v", err)
	}

	if output == "" {
		t.Error("notes should produce output")
	}
}

func TestRootCommand(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	gitClient := &mockGitClient{}
	githubClient := &mockGitHubClient{}

	cmd := Root(gitClient, githubClient, logger)

	if cmd == nil {
		t.Error("Root should return a command")
	}

	if cmd.Use != "semrel" {
		t.Errorf("Root command should be 'semrel', got: %s", cmd.Use)
	}

	// Check that subcommands are registered
	subcommands := []string{"lint", "release", "notify", "notes"}
	for _, subcmd := range subcommands {
		found := false
		for _, cmd := range cmd.Commands() {
			if cmd.Name() == subcmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Root should have subcommand: %s", subcmd)
		}
	}
}

func TestOutputReleaseFields(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "semrel-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	version := semver.Version{Major: 1, Minor: 2, Patch: 3}
	err = outputReleaseFields(tmpfile.Name(), version, true)
	if err != nil {
		t.Errorf("outputReleaseFields should not error, got: %v", err)
	}

	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	output := string(content)
	if !bytes.Contains([]byte(output), []byte("version=1.2.3")) {
		t.Errorf("output should contain version, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("tag=v1.2.3")) {
		t.Errorf("output should contain tag, got: %s", output)
	}
}

func TestGenerateReleaseNotes(t *testing.T) {
	commits := []conventional.Commit{
		{
			Type:        conventional.TypeFeat,
			Description: "add feature",
			SHA:         "abc123",
		},
		{
			Type:        conventional.TypeFix,
			Description: "fix bug",
			SHA:         "def456",
		},
	}

	prMap := map[string]notes.PR{
		"abc123": {Number: 1, URL: "https://github.com/owner/repo/pull/1"},
		"def456": {Number: 2, URL: "https://github.com/owner/repo/pull/2"},
	}

	result := generateReleaseNotes(commits, prMap)
	if result == "" {
		t.Error("generateReleaseNotes should produce output")
	}

	if !bytes.Contains([]byte(result), []byte("add feature")) {
		t.Errorf("release notes should contain commit description, got: %s", result)
	}
}
