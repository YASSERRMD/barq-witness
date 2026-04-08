// Package diff wraps go-git for extracting changed file/line information
// between two commits in a local repository.
package diff

import (
	"fmt"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// FileChange describes one changed file in a commit range.
type FileChange struct {
	// Path is the new (or current) file path.
	Path string
	// OldPath is the old file path (only differs from Path on renames).
	OldPath string
	// AddedLines is the 1-based line numbers that were added.
	AddedLines []int
	// DeletedLines is the 1-based line numbers that were deleted.
	DeletedLines []int
	// IsNew is true if the file did not exist in the from-commit.
	IsNew bool
	// IsDeleted is true if the file was removed.
	IsDeleted bool
}

// ChangedFiles returns the set of file changes between fromSHA and toSHA
// in the git repository rooted at repoPath.
// If fromSHA is empty, toSHA is compared against its first parent (or the
// empty tree for the initial commit).
func ChangedFiles(repoPath, fromSHA, toSHA string) ([]FileChange, error) {
	repo, err := gogit.PlainOpenWithOptions(repoPath, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	toCommit, err := resolveCommit(repo, toSHA)
	if err != nil {
		return nil, fmt.Errorf("resolve toSHA %q: %w", toSHA, err)
	}

	var fromTree *object.Tree
	if fromSHA != "" {
		fromCommit, err := resolveCommit(repo, fromSHA)
		if err != nil {
			return nil, fmt.Errorf("resolve fromSHA %q: %w", fromSHA, err)
		}
		fromTree, err = fromCommit.Tree()
		if err != nil {
			return nil, fmt.Errorf("from tree: %w", err)
		}
	} else {
		// Compare against first parent if available.
		if toCommit.NumParents() > 0 {
			parent, err := toCommit.Parent(0)
			if err != nil {
				return nil, fmt.Errorf("parent commit: %w", err)
			}
			fromTree, err = parent.Tree()
			if err != nil {
				return nil, fmt.Errorf("parent tree: %w", err)
			}
		}
		// fromTree stays nil for the initial commit (empty tree diff).
	}

	toTree, err := toCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("to tree: %w", err)
	}

	changes, err := object.DiffTree(fromTree, toTree)
	if err != nil {
		return nil, fmt.Errorf("diff tree: %w", err)
	}

	var result []FileChange
	for _, ch := range changes {
		fc, err := fileChangeFromGit(ch)
		if err != nil {
			// Non-fatal: skip binary or un-diffable files.
			continue
		}
		result = append(result, fc)
	}
	return result, nil
}

// ChangedFilePaths returns just the list of file paths changed between
// fromSHA and toSHA (convenience wrapper around ChangedFiles).
func ChangedFilePaths(repoPath, fromSHA, toSHA string) ([]string, error) {
	changes, err := ChangedFiles(repoPath, fromSHA, toSHA)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(changes))
	for _, c := range changes {
		paths = append(paths, c.Path)
	}
	return paths, nil
}

// resolveCommit looks up a commit by full or abbreviated SHA.
func resolveCommit(repo *gogit.Repository, sha string) (*object.Commit, error) {
	h := plumbing.NewHash(sha)
	commit, err := repo.CommitObject(h)
	if err == nil {
		return commit, nil
	}
	// Try resolving as a ref name (e.g. "HEAD", branch name).
	ref, err2 := repo.ResolveRevision(plumbing.Revision(sha))
	if err2 != nil {
		return nil, fmt.Errorf("not a commit or ref: %v / %v", err, err2)
	}
	return repo.CommitObject(*ref)
}

// fileChangeFromGit converts a go-git object.Change into our FileChange.
func fileChangeFromGit(ch *object.Change) (FileChange, error) {
	action, err := ch.Action()
	if err != nil {
		return FileChange{}, err
	}

	fc := FileChange{}

	switch action {
	case merkletrie.Insert:
		fc.IsNew = true
		fc.Path = ch.To.Name
		fc.OldPath = ch.To.Name
		lines, err := countLines(ch.To)
		if err != nil {
			return fc, err
		}
		fc.AddedLines = makeRange(1, lines)

	case merkletrie.Delete:
		fc.IsDeleted = true
		fc.Path = ch.From.Name
		fc.OldPath = ch.From.Name

	case merkletrie.Modify:
		fc.Path = ch.To.Name
		fc.OldPath = ch.From.Name
		added, deleted, err := diffLines(ch)
		if err != nil {
			// Non-fatal -- return fc without line info.
			return fc, nil //nolint:nilerr
		}
		fc.AddedLines = added
		fc.DeletedLines = deleted
	}

	return fc, nil
}

// diffLines computes the 1-based line numbers added and deleted in a change.
func diffLines(ch *object.Change) (added, deleted []int, err error) {
	patch, err := ch.Patch()
	if err != nil {
		return nil, nil, err
	}

	for _, fp := range patch.FilePatches() {
		chunks := fp.Chunks()
		oldLine := 0
		newLine := 0
		for _, chunk := range chunks {
			lines := strings.Split(chunk.Content(), "\n")
			// Remove trailing empty element caused by trailing newline.
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			switch chunk.Type() {
			case 0: // Equal
				oldLine += len(lines)
				newLine += len(lines)
			case 1: // Add
				for i := range lines {
					added = append(added, newLine+i+1)
				}
				newLine += len(lines)
			case 2: // Delete
				for i := range lines {
					deleted = append(deleted, oldLine+i+1)
				}
				oldLine += len(lines)
			}
		}
	}
	return added, deleted, nil
}

// countLines returns the number of lines in a TreeEntry blob.
func countLines(entry object.ChangeEntry) (int, error) {
	if entry.TreeEntry.Mode == 0 {
		return 0, nil
	}
	blob, err := entry.Tree.TreeEntryFile(&entry.TreeEntry)
	if err != nil {
		return 0, err
	}
	content, err := blob.Contents()
	if err != nil {
		return 0, err
	}
	return strings.Count(content, "\n") + 1, nil
}

// makeRange returns [start, start+1, ..., end].
func makeRange(start, end int) []int {
	if end < start {
		return nil
	}
	r := make([]int, end-start+1)
	for i := range r {
		r[i] = start + i
	}
	return r
}
