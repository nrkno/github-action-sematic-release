package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	gogithub "github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// Typed errors for GitHub API responses.
var (
	ErrUnauthorized   = errors.New("unauthorized: GITHUB_TOKEN is missing or expired")
	ErrForbidden      = errors.New("forbidden: GITHUB_TOKEN lacks required permissions")
	ErrNotFound       = errors.New("not found: resource does not exist")
	ErrUnprocessable  = errors.New("unprocessable: GitHub API rejected request")
	ErrRateLimited    = errors.New("rate limited: GitHub API rate limit exceeded")
	ErrServerError    = errors.New("server error: GitHub API returned 5xx")
)

// Client wraps the go-github client.
type Client struct {
	client *gogithub.Client
}

// PR represents a GitHub pull request.
type PR struct {
	Number int
	URL    string
	Title  string
}

// Release represents a GitHub release.
type Release struct {
	ID      int64
	TagName string
	HTMLURL string
	Body    string
}

// CreateReleaseOptions for CreateRelease.
type CreateReleaseOptions struct {
	TagName string
	Name    string
	Body    string
}

// NewClient creates a new GitHub API client.
// token: GitHub token for authentication
// baseURL: API base URL (e.g., "https://api.github.com/"). If empty, uses GitHub public API.
func NewClient(token, baseURL string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	var ghClient *gogithub.Client
	if baseURL != "" {
		var err error
		ghClient, err = gogithub.NewClient(httpClient).WithEnterpriseURLs(baseURL, baseURL)
		if err != nil {
			// If enterprise URL setup fails, fall back to default
			ghClient = gogithub.NewClient(httpClient)
		}
	} else {
		ghClient = gogithub.NewClient(httpClient)
	}

	return &Client{client: ghClient}
}

// GetReleaseByTag retrieves a release by tag name.
// Returns (Release, nil) on 200, typed errors on other status codes.
func (c *Client) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*Release, error) {
	rel, resp, err := c.client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return nil, mapHTTPError(resp.Response, err)
	}

	return &Release{
		ID:      rel.GetID(),
		TagName: rel.GetTagName(),
		HTMLURL: rel.GetHTMLURL(),
		Body:    rel.GetBody(),
	}, nil
}

// CreateRelease creates a new GitHub release.
// On HTTP 422 with errors[].code == "already_exists", calls GetReleaseByTag and returns that result.
// Returns (Release, nil) on success, typed errors on other failures.
func (c *Client) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*Release, error) {
	input := &gogithub.RepositoryRelease{
		TagName: gogithub.String(opts.TagName),
		Name:    gogithub.String(opts.Name),
		Body:    gogithub.String(opts.Body),
	}

	rel, resp, err := c.client.Repositories.CreateRelease(ctx, owner, repo, input)
	if err != nil {
		// Check if this is a 422 with already_exists error code
		if resp != nil && resp.Response != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			if isAlreadyExists(resp.Response) {
				// Recover by fetching the existing release
				return c.GetReleaseByTag(ctx, owner, repo, opts.TagName)
			}
		}
		return nil, mapHTTPError(resp.Response, err)
	}

	return &Release{
		ID:      rel.GetID(),
		TagName: rel.GetTagName(),
		HTMLURL: rel.GetHTMLURL(),
		Body:    rel.GetBody(),
	}, nil
}

// ListPRsForCommit retrieves all PRs associated with a commit SHA.
// Calls GET /repos/{owner}/{repo}/commits/{sha}/pulls
// Returns ([]PR, nil) on success, typed errors on failure.
func (c *Client) ListPRsForCommit(ctx context.Context, owner, repo, sha string) ([]PR, error) {
	opts := &gogithub.ListOptions{PerPage: 100}
	result := []PR{} // Initialize to empty slice, not nil

	for {
		prs, resp, err := c.client.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, sha, opts)
		if err != nil {
			return nil, mapHTTPError(resp.Response, err)
		}

		for _, pr := range prs {
			result = append(result, PR{
				Number: pr.GetNumber(),
				URL:    pr.GetHTMLURL(),
				Title:  pr.GetTitle(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return result, nil
}

// SearchPRsForCommit searches for PRs using a query string.
// Calls GET /search/issues?q=<query>
// Returns ([]PR, nil) on success, typed errors on failure.
func (c *Client) SearchPRsForCommit(ctx context.Context, query string) ([]PR, error) {
	opts := &gogithub.SearchOptions{
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}
	result := []PR{} // Initialize to empty slice, not nil

	for {
		searchResult, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, mapHTTPError(resp.Response, err)
		}

		for _, issue := range searchResult.Issues {
			result = append(result, PR{
				Number: issue.GetNumber(),
				URL:    issue.GetHTMLURL(),
				Title:  issue.GetTitle(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return result, nil
}

// PostPRComment posts a comment on a pull request.
// Returns nil on success, typed errors on failure.
func (c *Client) PostPRComment(ctx context.Context, owner, repo string, prNumber int, body string) error {
	_, resp, err := c.client.Issues.CreateComment(ctx, owner, repo, prNumber, &gogithub.IssueComment{
		Body: gogithub.String(body),
	})
	if err != nil {
		return mapHTTPError(resp.Response, err)
	}
	return nil
}

// FindPRComment searches for an existing comment on a PR by marker string.
// marker: substring to search for in comment body (e.g., "<!-- semrel-notify:v1.4.0 -->")
// Returns (true, nil) if found, (false, nil) if not found, (false, error) on API failure.
func (c *Client) FindPRComment(ctx context.Context, owner, repo string, prNumber int, marker string) (bool, error) {
	opts := &gogithub.IssueListCommentsOptions{
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := c.client.Issues.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return false, mapHTTPError(resp.Response, err)
		}

		for _, comment := range comments {
			if strings.Contains(comment.GetBody(), marker) {
				return true, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return false, nil
}

// mapHTTPError converts HTTP status codes and errors to typed errors.
func mapHTTPError(resp *http.Response, err error) error {
	if resp == nil {
		// Network or other error
		if err != nil {
			return fmt.Errorf("github api error: %w", err)
		}
		return ErrServerError
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnprocessableEntity:
		return ErrUnprocessable
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return ErrServerError
	default:
		if resp.StatusCode >= 500 {
			return ErrServerError
		}
		if err != nil {
			return fmt.Errorf("github api error: %w", err)
		}
		return fmt.Errorf("github api error: status %d", resp.StatusCode)
	}
}

// isAlreadyExists checks if the 422 response contains an "already_exists" error code.
func isAlreadyExists(resp *http.Response) bool {
	if resp == nil || resp.Body == nil {
		return false
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Reset the body for potential re-reading
	resp.Body = io.NopCloser(strings.NewReader(string(body)))

	// Parse the error response
	var errResp struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return false
	}

	// Check if any error has code "already_exists"
	for _, e := range errResp.Errors {
		if e.Code == "already_exists" {
			return true
		}
	}

	return false
}
