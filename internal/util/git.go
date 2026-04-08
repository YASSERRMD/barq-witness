package util

import (
	"fmt"

	gogit "github.com/go-git/go-git/v5"
)

// HeadSHA returns the full SHA-1 hash of HEAD in the git repository
// rooted at repoPath. Returns an empty string (not an error) if the
// repository has no commits yet (e.g. freshly initialised).
func HeadSHA(repoPath string) (string, error) {
	repo, err := gogit.PlainOpenWithOptions(repoPath, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return "", fmt.Errorf("open git repo at %q: %w", repoPath, err)
	}

	ref, err := repo.Head()
	if err != nil {
		// Unborn HEAD (no commits yet) -- not a hard error for our purposes.
		return "", nil //nolint:nilerr
	}

	return ref.Hash().String(), nil
}
