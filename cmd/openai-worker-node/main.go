// Package main is the openai-worker-node binary entrypoint.
//
// Wiring is assembled here: parse --config, dial the PayeeDaemon unix
// socket, cross-check ListCapabilities, register capability modules,
// start the HTTP server.
//
// This file is a placeholder landed by plan 0001-repo-scaffold. The real
// wiring arrives with plan 0003-payment-middleware; individual capability
// modules arrive in 0002 (chat), 0004 (embeddings), 0005 (images), 0006
// (audio). Keep this file as a one-line main that calls into a testable
// runtime/ package — do not grow it.
package main

func main() {
	// TODO(0003-payment-middleware): wire runtime.Run(context.Background())
}
