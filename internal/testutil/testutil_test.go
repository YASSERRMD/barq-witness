package testutil_test

import (
	"testing"

	"github.com/yasserrmd/barq-witness/internal/testutil"
)

func TestNewFixtureStore_OpenAndClose(t *testing.T) {
	st := testutil.NewFixtureStore(t)
	if st == nil {
		t.Fatal("expected non-nil store")
	}
	// Cleanup is registered via t.Cleanup; no explicit close needed here.
}

func TestNewFixtureRepo_ReturnsTwoSHAs(t *testing.T) {
	repoPath, parentSHA, headSHA := testutil.NewFixtureRepo(t)
	if repoPath == "" {
		t.Error("expected non-empty repoPath")
	}
	if parentSHA == "" {
		t.Error("expected non-empty parentSHA")
	}
	if headSHA == "" {
		t.Error("expected non-empty headSHA")
	}
	if parentSHA == headSHA {
		t.Errorf("parentSHA and headSHA should differ, got %q", parentSHA)
	}
}

func TestNewFixtureRepo_SHALength(t *testing.T) {
	_, parentSHA, headSHA := testutil.NewFixtureRepo(t)
	// Full SHA-1 is 40 hex characters.
	if len(parentSHA) != 40 {
		t.Errorf("parentSHA length = %d, want 40", len(parentSHA))
	}
	if len(headSHA) != 40 {
		t.Errorf("headSHA length = %d, want 40", len(headSHA))
	}
}

func TestNewFixtureSession_InsertsSession(t *testing.T) {
	st := testutil.NewFixtureStore(t)
	sessionID := testutil.NewFixtureSession(t, st)
	if sessionID == "" {
		t.Fatal("expected non-empty sessionID")
	}

	sessions, err := st.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.ID == sessionID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("session %q not found after insert", sessionID)
	}
}
