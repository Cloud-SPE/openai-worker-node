// Package config parses, validates, and projects the shared
// worker.yaml into the worker-specific runtime form. The worker and
// receiver-mode payment daemon both consume this file; the worker
// validates the worker-facing fields, captures payment_daemon
// opaquely, and projects the capability catalog.
//
// The package also owns the daemon-consistency cross-check: once the
// worker dials the payee daemon and pulls ListCapabilities, call
// VerifyDaemonCatalog to assert byte-equality against the worker's own
// parse. Mismatch is fail-closed.
package config
