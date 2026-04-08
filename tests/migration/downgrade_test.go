package migration_test

import (
	"testing"
)

// TestDowngrade_Documented documents that downgrading (opening a migrated DB
// with an older version of barq-witness that does not know the newer schema)
// is NOT supported.
//
// Since barq-witness was built in a single development pass with no real
// historical binary releases, there is no tooling to produce a "downgraded"
// binary. This test serves as a documentation-only pass and records the policy
// decision in the test output.
//
// Policy: once a database has been migrated to schema version N, opening it
// with software that implements only schema version < N may produce errors or
// silent data loss. Users must not attempt to downgrade. Backups should be
// taken before upgrades.
func TestDowngrade_Documented(t *testing.T) {
	t.Log("POLICY: barq-witness does not support schema downgrade.")
	t.Log("Once store.Open has applied migrations (schema_version=2 as of v1.0),")
	t.Log("the database cannot be safely opened by a binary built before those")
	t.Log("migrations were introduced. No downgrade path is provided.")
	t.Log("Recommendation: back up .witness/trace.db before upgrading barq-witness.")
	// This test is intentionally documentation-only. It always passes.
}
