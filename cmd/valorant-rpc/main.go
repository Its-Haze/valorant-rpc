// Command valorant-rpc runs the always-on background daemon. Dev and debug
// only; the shipped binary is cmd/valorant-rpc-gui.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/its-haze/valorant-rpc/internal/config"
	"github.com/its-haze/valorant-rpc/internal/daemon"
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
	store := config.NewStore(cfg)
	d, _ := daemon.Wire(store, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info().Str("app", constants.AppName).Msg("starting")
	d.Run(ctx)
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
