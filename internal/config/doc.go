// Package config parses, validates, and projects worker.yaml into the
// worker-specific form. In v3 the worker owns this config file
// outright; the payment daemon parses a separate payment-daemon.yaml.
// Validation covers worker settings and the capability catalog.
//
// The package also owns the daemon-consistency cross-check: once the
// worker dials the payee daemon and pulls ListCapabilities, call
// VerifyDaemonCatalog to assert byte-equality against the worker's own
// parse. Mismatch is fail-closed.
package config
