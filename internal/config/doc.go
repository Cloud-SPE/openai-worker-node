// Package config parses, validates, and projects worker.yaml into the
// worker-specific form. Validation covers protocol_version, the worker
// section, and the shared capabilities catalog; the daemon section is
// accepted (so a single shared worker.yaml file can serve both
// processes) but not validated here — the daemon validates its own
// section and refuses to start on errors.
//
// The package also owns the daemon-consistency cross-check: once the
// worker dials the payee daemon and pulls ListCapabilities, call
// VerifyDaemonCatalog to assert byte-equality against the worker's
// own parse. Mismatch is fail-closed.
package config
