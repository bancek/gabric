// Package gabrice2e provides high-level end-to-end test targets for Gabric.
//
// It runs real Docker containers, SSH servers, LocalRemote targets, and plain
// local targets, and it keeps image builds, container lifecycle, readiness, and
// cleanup together. It does not provide a matcher DSL; tests use the normal
// Gabric operations.
package gabrice2e
