package git

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Commit represents a git commit
type Commit struct {
	SHA      string    // full commit SHA
	ShortSHA string    // first 7 chars
	Author   string
	Date     time.Time
	Message  string
}

// Tag represents an annotated git tag
type Tag struct {
	Name      string // tag name (e.g., "v1.0.0")
	SHA       string // tag object SHA (for annotated tags)
	targetSHA string // commit SHA that this tag points to (cached)
}

// TargetSHA returns the commit SHA that this tag points to (distinct from tag object SHA)
func (t *Tag) TargetSHA() string {
	return t.targetSHA
}

// Repository wraps a go-git repository
type Repository struct {
	raw *gogit.Repository
}

// ShallowRepoError is returned when a shallow clone is detected
type ShallowRepoError struct {
	Message string
}

func (e ShallowRepoError) Error() string {
	return e.Message
}

// BasicAuth holds HTTPS authentication credentials
type BasicAuth struct {
	Username string
	Password string
}

// OpenRepo opens a git repository at the given path.
// Returns ShallowRepoError if the repo is a shallow clone.
func OpenRepo(path string) (*Repository, error) {
	// Check for .git/shallow file first
	shallowPath := path + "/.git/shallow"
	if _, err := os.Stat(shallowPath); err == nil {
		return nil, ShallowRepoError{Message: "repository is a shallow clone"}
	}

	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Check if go-git detects a shallow repository via ShallowStorer
	shallow, err := repo.Storer.(storer.ShallowStorer).Shallow()
	if err == nil && len(shallow) > 0 {
		return nil, ShallowRepoError{Message: "repository is a shallow clone"}
	}

	return &Repository{raw: repo}, nil
}

// FindLatestAnnotatedTag finds the latest annotated tag in the repository.
// Returns nil, nil if no annotated tags exist (bootstrap case).
func (r *Repository) FindLatestAnnotatedTag() (*Tag, error) {
	tags, err := r.raw.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	var annotatedTags []*Tag
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		// Only process annotated tags (tag objects), not lightweight tags
		obj, err := r.raw.TagObject(ref.Hash())
		if err != nil {
			// Not an annotated tag (lightweight tag), skip
			return nil
		}

		// Get the target commit SHA
		targetSHA := obj.Target.String()

		tag := &Tag{
			Name:      ref.Name().Short(),
			SHA:       obj.Hash.String(),
			targetSHA: targetSHA,
		}
		annotatedTags = append(annotatedTags, tag)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate tags: %w", err)
	}

	if len(annotatedTags) == 0 {
		return nil, nil
	}

	// Sort by target commit date (most recent first)
	sort.Slice(annotatedTags, func(i, j int) bool {
		// Get the commit objects to compare dates
		commitI, errI := r.raw.CommitObject(plumbing.NewHash(annotatedTags[i].targetSHA))
		commitJ, errJ := r.raw.CommitObject(plumbing.NewHash(annotatedTags[j].targetSHA))
		if errI != nil || errJ != nil {
			// Fallback: keep original order if we can't get commits
			return false
		}
		return commitI.Author.When.After(commitJ.Author.When)
	})

	return annotatedTags[0], nil
}

// FindPreviousAnnotatedTag finds the annotated tag before the given tag.
// Returns nil, nil if the given tag is the only tag.
func (r *Repository) FindPreviousAnnotatedTag(current *Tag) (*Tag, error) {
	tags, err := r.raw.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	var annotatedTags []*Tag
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		// Only process annotated tags (tag objects), not lightweight tags
		obj, err := r.raw.TagObject(ref.Hash())
		if err != nil {
			// Not an annotated tag (lightweight tag), skip
			return nil
		}

		// Get the target commit SHA
		targetSHA := obj.Target.String()

		tag := &Tag{
			Name:      ref.Name().Short(),
			SHA:       obj.Hash.String(),
			targetSHA: targetSHA,
		}
		annotatedTags = append(annotatedTags, tag)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate tags: %w", err)
	}

	// Sort by target commit date (most recent first)
	sort.Slice(annotatedTags, func(i, j int) bool {
		commitI, errI := r.raw.CommitObject(plumbing.NewHash(annotatedTags[i].targetSHA))
		commitJ, errJ := r.raw.CommitObject(plumbing.NewHash(annotatedTags[j].targetSHA))
		if errI != nil || errJ != nil {
			return false
		}
		return commitI.Author.When.After(commitJ.Author.When)
	})

	// Find the current tag in the sorted list
	currentIdx := -1
	for i, tag := range annotatedTags {
		if tag.SHA == current.SHA {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 || currentIdx == len(annotatedTags)-1 {
		// Current tag not found or is the last (oldest) tag
		return nil, nil
	}

	return annotatedTags[currentIdx+1], nil
}

// ListCommitsSinceTag lists all commits from HEAD back to (but not including) the tag's target commit.
// If tag is nil, returns all commits from HEAD (bootstrap case).
// Returns commits in reverse-chronological order (newest first).
func (r *Repository) ListCommitsSinceTag(tag *Tag) ([]Commit, error) {
	head, err := r.raw.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	iter, err := r.raw.Log(&gogit.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("failed to create log iterator: %w", err)
	}
	defer iter.Close()

	var commits []Commit
	var stopHash plumbing.Hash
	if tag != nil {
		stopHash = plumbing.NewHash(tag.TargetSHA())
	}

	err = iter.ForEach(func(c *object.Commit) error {
		// Skip the tag's target commit itself
		if tag != nil && c.Hash == stopHash {
			return storer.ErrStop
		}

		commit := Commit{
			SHA:      c.Hash.String(),
			ShortSHA: c.Hash.String()[:7],
			Author:   c.Author.Name,
			Date:     c.Author.When,
			Message:  c.Message,
		}
		commits = append(commits, commit)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// ListCommitsBetweenTags lists commits between two tags (exclusive of from, inclusive of to).
// Returns commits in reverse-chronological order.
func (r *Repository) ListCommitsBetweenTags(from, to *Tag) ([]Commit, error) {
	toHash := plumbing.NewHash(to.TargetSHA())

	iter, err := r.raw.Log(&gogit.LogOptions{From: toHash})
	if err != nil {
		return nil, fmt.Errorf("failed to create log iterator: %w", err)
	}
	defer iter.Close()

	var commits []Commit
	fromHash := plumbing.NewHash(from.TargetSHA())

	err = iter.ForEach(func(c *object.Commit) error {
		// Stop when we reach the from commit (exclusive)
		if c.Hash == fromHash {
			return storer.ErrStop
		}

		commit := Commit{
			SHA:      c.Hash.String(),
			ShortSHA: c.Hash.String()[:7],
			Author:   c.Author.Name,
			Date:     c.Author.When,
			Message:  c.Message,
		}
		commits = append(commits, commit)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// CreateAnnotatedTag creates an annotated tag at HEAD.
// message: tag message (e.g., "chore(release): 1.0.0")
func (r *Repository) CreateAnnotatedTag(name, message string) (*Tag, error) {
	head, err := r.raw.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get the commit object to extract author info
	commit, err := r.raw.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	// Create annotated tag
	ref, err := r.raw.CreateTag(name, head.Hash(), &gogit.CreateTagOptions{
		Tagger: &object.Signature{
			Name:  commit.Author.Name,
			Email: commit.Author.Email,
			When:  time.Now(),
		},
		Message: message,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}

	// Get the tag object to extract SHA
	tagObj, err := r.raw.TagObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get tag object: %w", err)
	}

	tag := &Tag{
		Name:      name,
		SHA:       tagObj.Hash.String(),
		targetSHA: tagObj.Target.String(),
	}

	return tag, nil
}

// PushTag pushes a tag to the remote repository.
// auth: BasicAuth{Username, Password} for HTTPS authentication
func (r *Repository) PushTag(ctx context.Context, tagName string, auth BasicAuth) error {
	// Create go-git BasicAuth from our BasicAuth
	gitAuth := &http.BasicAuth{
		Username: auth.Username,
		Password: auth.Password,
	}

	err := r.raw.PushContext(ctx, &gogit.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("refs/tags/%s:refs/tags/%s", tagName, tagName)),
		},
		Auth: gitAuth,
	})
	if err != nil {
		return fmt.Errorf("failed to push tag: %w", err)
	}

	return nil
}
