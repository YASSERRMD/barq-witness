package store_test

import (
	"testing"
	"time"

	"github.com/yasserrmd/barq-witness/internal/model"
)

// TestAllSessions_Empty returns an empty (not nil) result on an empty store.
func TestAllSessions_Empty(t *testing.T) {
	s := newTestStore(t)
	sessions, err := s.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	// len(nil) == 0 so either nil or empty slice is fine.
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions on empty store, got %d", len(sessions))
	}
}

// TestAllSessions_OrderedAscending verifies sessions are returned in ascending order.
func TestAllSessions_OrderedAscending(t *testing.T) {
	s := newTestStore(t)
	base := time.Now().UnixMilli()

	for i := 0; i < 3; i++ {
		sess := model.Session{
			ID:        "sess-order-" + string(rune('a'+i)),
			StartedAt: base + int64(i)*1000,
		}
		if err := s.InsertSession(sess); err != nil {
			t.Fatalf("InsertSession: %v", err)
		}
	}

	sessions, err := s.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	for i := 1; i < len(sessions); i++ {
		if sessions[i].StartedAt < sessions[i-1].StartedAt {
			t.Errorf("sessions not in ascending order at index %d", i)
		}
	}
}

// TestAllSessions_SourceDefaulted verifies sessions with empty source get "claude-code".
func TestAllSessions_SourceDefaulted(t *testing.T) {
	s := newTestStore(t)

	// Insert with empty source.
	if err := s.InsertSession(model.Session{ID: "sess-src", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	sessions, err := s.AllSessions()
	if err != nil {
		t.Fatalf("AllSessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least one session")
	}

	for _, sess := range sessions {
		if sess.ID == "sess-src" && sess.Source != "claude-code" {
			t.Errorf("expected source='claude-code', got %q", sess.Source)
		}
	}
}

// TestPromptsForSession_Empty returns empty when no prompts exist.
func TestPromptsForSession_Empty(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-ps-empty", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	prompts, err := s.PromptsForSession("sess-ps-empty")
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected 0 prompts, got %d", len(prompts))
	}
}

// TestPromptsForSession_MultiplePrompts verifies all prompts are returned in order.
func TestPromptsForSession_MultiplePrompts(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-ps-multi", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	base := nowMS()
	for i := 0; i < 3; i++ {
		_, err := s.InsertPrompt(model.Prompt{
			SessionID:   "sess-ps-multi",
			Timestamp:   base + int64(i)*100,
			Content:     "prompt number " + string(rune('0'+i)),
			ContentHash: "hash" + string(rune('a'+i)),
		})
		if err != nil {
			t.Fatalf("InsertPrompt %d: %v", i, err)
		}
	}

	prompts, err := s.PromptsForSession("sess-ps-multi")
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(prompts))
	}

	for i := 1; i < len(prompts); i++ {
		if prompts[i].Timestamp < prompts[i-1].Timestamp {
			t.Errorf("prompts not in ascending order at index %d", i)
		}
	}
}

// TestPromptsForSession_WrongSession returns empty for non-matching session.
func TestPromptsForSession_WrongSession(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-a", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession sess-a: %v", err)
	}
	if _, err := s.InsertPrompt(model.Prompt{
		SessionID:   "sess-a",
		Timestamp:   nowMS(),
		Content:     "some prompt",
		ContentHash: "abc",
	}); err != nil {
		t.Fatalf("InsertPrompt: %v", err)
	}

	prompts, err := s.PromptsForSession("sess-b")
	if err != nil {
		t.Fatalf("PromptsForSession: %v", err)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected 0 prompts for wrong session, got %d", len(prompts))
	}
}

// TestPromptByID_Found verifies a prompt can be retrieved by its ID.
func TestPromptByID_Found(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-pid", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	id, err := s.InsertPrompt(model.Prompt{
		SessionID:   "sess-pid",
		Timestamp:   nowMS(),
		Content:     "find me by id",
		ContentHash: "findme",
	})
	if err != nil {
		t.Fatalf("InsertPrompt: %v", err)
	}

	p, err := s.PromptByID(id)
	if err != nil {
		t.Fatalf("PromptByID: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil prompt")
	}
	if p.ID != id {
		t.Errorf("PromptByID returned id=%d, want %d", p.ID, id)
	}
	if p.Content != "find me by id" {
		t.Errorf("PromptByID content = %q, want 'find me by id'", p.Content)
	}
}

// TestPromptByID_NotFound returns nil for a missing id.
func TestPromptByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	p, err := s.PromptByID(999999)
	if err != nil {
		t.Fatalf("PromptByID: %v", err)
	}
	if p != nil {
		t.Fatalf("expected nil for missing id, got %+v", p)
	}
}

// TestEditsForSession_Empty returns empty when no edits exist.
func TestEditsForSession_Empty(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-efs-empty", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	edits, err := s.EditsForSession("sess-efs-empty")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 0 {
		t.Fatalf("expected 0 edits, got %d", len(edits))
	}
}

// TestEditsForSession_MultipleEdits returns all edits in ascending timestamp order.
func TestEditsForSession_MultipleEdits(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-efs-multi", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	base := nowMS()
	files := []string{"a.go", "b.go", "c.go"}
	for i, f := range files {
		if err := s.InsertEdit(model.Edit{
			SessionID: "sess-efs-multi",
			Timestamp: base + int64(i)*100,
			FilePath:  f,
			Tool:      "Write",
		}); err != nil {
			t.Fatalf("InsertEdit %d: %v", i, err)
		}
	}

	edits, err := s.EditsForSession("sess-efs-multi")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 3 {
		t.Fatalf("expected 3 edits, got %d", len(edits))
	}

	for i := 1; i < len(edits); i++ {
		if edits[i].Timestamp < edits[i-1].Timestamp {
			t.Errorf("edits not in ascending order at index %d", i)
		}
	}
}

// TestEditsForSession_WrongSession returns empty for non-matching session.
func TestEditsForSession_WrongSession(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-efs-right", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}
	if err := s.InsertEdit(model.Edit{
		SessionID: "sess-efs-right",
		Timestamp: nowMS(),
		FilePath:  "main.go",
		Tool:      "Edit",
	}); err != nil {
		t.Fatalf("InsertEdit: %v", err)
	}

	edits, err := s.EditsForSession("sess-efs-wrong")
	if err != nil {
		t.Fatalf("EditsForSession: %v", err)
	}
	if len(edits) != 0 {
		t.Fatalf("expected 0 edits for wrong session, got %d", len(edits))
	}
}

// TestEditByID_Found verifies EditByID returns the correct edit.
func TestEditByID_Found(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertSession(model.Session{ID: "sess-ebid", StartedAt: nowMS()}); err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	lineStart, lineEnd := 5, 15
	if err := s.InsertEdit(model.Edit{
		SessionID:  "sess-ebid",
		Timestamp:  nowMS(),
		FilePath:   "target.go",
		Tool:       "MultiEdit",
		BeforeHash: "bh123",
		AfterHash:  "ah456",
		LineStart:  &lineStart,
		LineEnd:    &lineEnd,
	}); err != nil {
		t.Fatalf("InsertEdit: %v", err)
	}

	// Get the edit we just inserted.
	edits, err := s.EditsForFiles([]string{"target.go"})
	if err != nil || len(edits) == 0 {
		t.Fatalf("EditsForFiles: err=%v edits=%d", err, len(edits))
	}
	editID := edits[0].ID

	e, err := s.EditByID(editID)
	if err != nil {
		t.Fatalf("EditByID: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil edit")
	}
	if e.FilePath != "target.go" {
		t.Errorf("EditByID.FilePath = %q, want target.go", e.FilePath)
	}
	if e.BeforeHash != "bh123" {
		t.Errorf("EditByID.BeforeHash = %q, want bh123", e.BeforeHash)
	}
}

// TestEditByID_NotFound returns nil for a missing edit id.
func TestEditByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	e, err := s.EditByID(999999)
	if err != nil {
		t.Fatalf("EditByID: %v", err)
	}
	if e != nil {
		t.Fatalf("expected nil for missing id, got %+v", e)
	}
}

// TestGetStats_WithData verifies GetStats returns correct counts.
func TestGetStats_WithData(t *testing.T) {
	s := newTestStore(t)

	// Insert 2 sessions and 3 edits.
	for i := 0; i < 2; i++ {
		if err := s.InsertSession(model.Session{
			ID:        "sess-stats-" + string(rune('a'+i)),
			StartedAt: nowMS(),
		}); err != nil {
			t.Fatalf("InsertSession %d: %v", i, err)
		}
	}

	for i := 0; i < 3; i++ {
		if err := s.InsertEdit(model.Edit{
			SessionID: "sess-stats-a",
			Timestamp: nowMS(),
			FilePath:  "file" + string(rune('0'+i)) + ".go",
			Tool:      "Write",
		}); err != nil {
			t.Fatalf("InsertEdit %d: %v", i, err)
		}
	}

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", stats.TotalSessions)
	}
	if stats.TotalEdits != 3 {
		t.Errorf("TotalEdits = %d, want 3", stats.TotalEdits)
	}
}

// TestGetStats_Empty returns zero counts on an empty store.
func TestGetStats_Empty(t *testing.T) {
	s := newTestStore(t)
	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalEdits != 0 {
		t.Errorf("TotalEdits = %d, want 0", stats.TotalEdits)
	}
	if stats.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", stats.TotalSessions)
	}
}

// TestRecentSessions_Limit verifies the limit parameter is respected.
func TestRecentSessions_Limit(t *testing.T) {
	s := newTestStore(t)
	base := time.Now().UnixMilli()

	for i := 0; i < 5; i++ {
		if err := s.InsertSession(model.Session{
			ID:        "sess-recent-" + string(rune('a'+i)),
			StartedAt: base + int64(i)*1000,
		}); err != nil {
			t.Fatalf("InsertSession %d: %v", i, err)
		}
	}

	sessions, err := s.RecentSessions(3)
	if err != nil {
		t.Fatalf("RecentSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
}

// TestRecentSessions_NoLimit returns all sessions when limit <= 0.
func TestRecentSessions_NoLimit(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 4; i++ {
		if err := s.InsertSession(model.Session{
			ID:        "sess-nolimit-" + string(rune('a'+i)),
			StartedAt: nowMS() + int64(i)*1000,
		}); err != nil {
			t.Fatalf("InsertSession %d: %v", i, err)
		}
	}

	sessions, err := s.RecentSessions(0)
	if err != nil {
		t.Fatalf("RecentSessions(0): %v", err)
	}
	if len(sessions) != 4 {
		t.Fatalf("expected 4 sessions, got %d", len(sessions))
	}
}
