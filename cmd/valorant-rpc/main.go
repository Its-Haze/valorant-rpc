// Command valorant-rpc runs the always-on background daemon. Dev and debug
// only; the shipped binary is cmd/valorant-rpc-gui.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/its-haze/valorant-rpc/internal/config"
	"github.com/its-haze/valorant-rpc/pkg/constants"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	logger := newLogger(cfg.Advanced.DebugMode)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info().Str("app", constants.AppName).Msg("starting")
	// Nothing to run yet: this waits for a signal until the daemon exists.
	<-ctx.Done()
	logger.Info().Str("app", constants.AppName).Msg("stopped")
}

func newLogger(debug bool) zerolog.Logger {
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}
	return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
		Level(level).
		With().Timestamp().Logger()
}
