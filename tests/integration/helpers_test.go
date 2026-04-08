package integration

import (
	"github.com/yasserrmd/barq-witness/internal/model"
)

// sessionFixture returns a minimal model.Session for use in migration and store tests.
func sessionFixture(id string) model.Session {
	return model.Session{
		ID:           id,
		StartedAt:    1700000000000,
		CWD:          "/tmp/test",
		GitHeadStart: "abc123",
		Model:        "claude-sonnet-4-6",
		Source:       "claude-code",
	}
}
