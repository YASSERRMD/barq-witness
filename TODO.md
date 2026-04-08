# TODO

Items identified during the v1.1.1 test hardening pass that were intentionally deferred.

## Plugin main() entry points not unit-testable

The no-prod-secrets and license-header-check plugin main() functions read from os.Stdin
and write to os.Stdout, making them hard to unit test. Coverage is 71-72%.
Fix: extract stdin/stdout to io.Reader/io.Writer parameters (minor refactor).

## cmd/barq-witness/ CLI coverage

The CLI surface (13 files) has no dedicated test coverage beyond integration tests.
The integration tests cover the happy path but not all error paths.
Fix: add table-driven tests with a mock store interface.

## Historical migration fixtures

No real historical binary releases exist to generate v0.1/v0.5 fixture DBs.
Only a synthetic v1.0 fixture is tested.
Fix: when v1.x users report upgrade issues, capture their DB as a fixture.

## Explainer live tests require credentials

All non-null explainer live tests are skipped in CI.
Fix: add a mock HTTP server for each explainer in unit tests (partially done in Phase C).
