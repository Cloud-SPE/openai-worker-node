// Package main is the openai-worker-node binary entrypoint.
//
// Startup sequence:
//  1. Parse --config; Load + Validate the shared worker.yaml.
//  2. Dial the payee daemon's unix socket.
//  3. ListCapabilities; cross-check against the parsed config
//     (unless worker.verify_daemon_consistency_on_start = false).
//  4. Build providers (backend HTTP, tokenizer).
//  5. Build the Mux; register unpaid handlers; register every
//     capability module whose capability is declared in the config.
//  6. Start HTTP server; block until SIGINT / SIGTERM; graceful
//     shutdown with a 30s deadline.
//
// All startup failures exit non-zero. No partial-start fallbacks.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Cloud-SPE/openai-worker-node/internal/config"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/backendhttp"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/payeedaemon"
	"github.com/Cloud-SPE/openai-worker-node/internal/providers/tokenizer"
	rthttp "github.com/Cloud-SPE/openai-worker-node/internal/runtime/http"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules/audio_speech"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules/chat_completions"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules/embeddings"
	"github.com/Cloud-SPE/openai-worker-node/internal/service/modules/images_generations"
	"github.com/Cloud-SPE/openai-worker-node/internal/types"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run is the testable entrypoint. Returns a process exit code:
//
//	0  — clean shutdown (signal received)
//	1  — runtime failure (listen error, dial/catalog mismatch, etc.)
//	2  — flag / usage error
func run(args []string, stderr *os.File) int {
	fs := flag.NewFlagSet("openai-worker-node", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "/etc/livepeer/worker.yaml", "path to the shared worker.yaml")
	logLevel := fs.String("log-level", "info", "minimum log level: error|warn|info|debug")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	logger := buildLogger(*logLevel, stderr)
	slog.SetDefault(logger)

	// 1. Load + validate config.
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config", "path", *configPath, "err", err)
		return 1
	}
	logger.Info("config loaded",
		"path", *configPath,
		"protocol_version", cfg.ProtocolVersion,
		"capabilities", len(cfg.Capabilities.Ordered),
		"http_listen", cfg.Worker.HTTPListen)

	// 2. Dial the payee daemon.
	payee, err := payeedaemon.Dial(cfg.Worker.PaymentDaemonSocket)
	if err != nil {
		logger.Error("dial payment daemon", "socket", cfg.Worker.PaymentDaemonSocket, "err", err)
		return 1
	}
	defer func() { _ = payee.Close() }()

	// 3. Cross-check catalog.
	if cfg.Worker.VerifyDaemonConsistencyOnStart {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		daemonCatalog, err := payee.ListCapabilities(ctx)
		if err != nil {
			logger.Error("ListCapabilities", "err", err)
			return 1
		}
		if err := config.VerifyDaemonCatalog(cfg, daemonCatalog); err != nil {
			logger.Error("daemon catalog mismatch — refusing to start", "err", err)
			return 1
		}
		logger.Info("daemon catalog verified", "capabilities", len(daemonCatalog.Capabilities))
	} else {
		logger.Warn("daemon consistency verification DISABLED by config — drift is possible",
			"hint", "flip worker.verify_daemon_consistency_on_start=true in worker.yaml for production")
	}

	// 4. Providers.
	tok := tokenizer.NewWordCount(133)
	backend := backendhttp.NewFetch()

	// 5. Mux + handlers + modules.
	mux := rthttp.NewMux(cfg, payee, logger)
	rthttp.RegisterUnpaidHandlers(mux, cfg)

	registered := registerModules(mux, cfg, tok, backend, logger)
	if registered == 0 {
		logger.Error("no capability modules registered — config has capabilities this build doesn't implement",
			"configured", cfg.Capabilities.Ordered)
		return 1
	}
	if missing := missingCapabilityModules(mux, cfg); len(missing) > 0 {
		logger.Error("config declares capabilities with no module backing",
			"missing", missing)
		return 1
	}

	// 6. Server.
	srv := rthttp.NewServer(mux, cfg.Worker.HTTPListen, logger)
	serverErr := make(chan error, 1)
	go func() { serverErr <- srv.Start() }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			logger.Error("server exited unexpectedly", "err", err)
			return 1
		}
		return 0
	case s := <-sig:
		logger.Info("received signal; beginning graceful shutdown", "signal", s.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		return 1
	}
	// Drain the Start goroutine.
	if err := <-serverErr; err != nil && !errors.Is(err, context.Canceled) {
		logger.Warn("server error during shutdown", "err", err)
	}
	return 0
}

// registerModules wires every capability module this binary supports.
// Only modules whose capability string appears in the config are
// registered — a build that supports a module the operator didn't
// configure stays quiet.
//
// Returns the number of modules actually registered. Add new modules
// by appending to the switch below; this is the one place where the
// binary's capability catalog is enumerated.
func registerModules(
	mux *rthttp.Mux,
	cfg *config.Config,
	tok tokenizer.Tokenizer,
	backend backendhttp.Client,
	logger *slog.Logger,
) int {
	registered := 0
	for _, entry := range cfg.Capabilities.Ordered {
		switch entry.Capability {
		case chat_completions.Capability:
			mod := chat_completions.New(tok, backend)
			mux.RegisterPaidRoute(mod)
			logger.Info("capability registered",
				"capability", mod.Capability(),
				"path", mod.HTTPPath(),
				"models", len(entry.Models))
			registered++
		case embeddings.Capability:
			mod := embeddings.New(tok, backend)
			mux.RegisterPaidRoute(mod)
			logger.Info("capability registered",
				"capability", mod.Capability(),
				"path", mod.HTTPPath(),
				"models", len(entry.Models))
			registered++
		case images_generations.Capability:
			mod := images_generations.New(backend)
			mux.RegisterPaidRoute(mod)
			logger.Info("capability registered",
				"capability", mod.Capability(),
				"path", mod.HTTPPath(),
				"models", len(entry.Models))
			registered++
		case audio_speech.Capability:
			mod := audio_speech.New(backend)
			mux.RegisterPaidRoute(mod)
			logger.Info("capability registered",
				"capability", mod.Capability(),
				"path", mod.HTTPPath(),
				"models", len(entry.Models))
			registered++
		default:
			logger.Warn("config declares capability this build doesn't implement",
				"capability", entry.Capability,
				"hint", "add a module under internal/service/modules/ and register it here")
		}
	}
	return registered
}

// missingCapabilityModules reports any configured capability whose
// module wasn't registered. This catches the case where the switch in
// registerModules falls through for a capability the operator declared
// — we'd rather refuse to start than silently half-serve the catalog.
func missingCapabilityModules(mux *rthttp.Mux, cfg *config.Config) []types.CapabilityID {
	var missing []types.CapabilityID
	for _, entry := range cfg.Capabilities.Ordered {
		if !mux.HasPaidCapability(entry.Capability) {
			missing = append(missing, entry.Capability)
		}
	}
	return missing
}

func buildLogger(level string, out *os.File) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "info":
		lv = slog.LevelInfo
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
		fmt.Fprintf(out, "unknown --log-level %q; defaulting to info\n", level)
	}
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: lv}))
}
