package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"

	"github.com/fly-hiring/52397/proxy/pkg/app"
	"github.com/fly-hiring/52397/proxy/pkg/server"
	flag "github.com/spf13/pflag"
	"golang.org/x/exp/slog"
)

func main() {
	var (
		// maxWorkers is the maximum number of workers that the server will use.
		maxWorkers int
		configFile string
	)
	flag.IntVar(&maxWorkers, "max-workers", 50, "maximum number of workers")
	flag.StringVar(&configFile, "config", "config.json", "config file containing apps")
	flag.Parse()

	textHandler := slog.NewTextHandler(os.Stdout).
		WithAttrs([]slog.Attr{slog.String("app-version", "v0.0.1")})
	logger := slog.New(textHandler)

	apps, err := loadConfigFile(configFile)
	if err != nil {
		logger.Error("Error loading config file", err)
		os.Exit(1)
	}

	// create a signal channel
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	server := run(ctx, &apps, maxWorkers, logger)

	<-sigc
	logger.Info("Caught interrupt signal, shutting down...")
	stop(cancel, server)
	logger.Info("Shutdown complete")
	os.Exit(0)
}

func loadConfigFile(path string) (app.Apps, error) {
	// load the apps from the config file
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return app.Apps{}, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return app.Apps{}, err
	}

	var apps app.Apps
	err = json.Unmarshal(content, &apps)
	if err != nil {
		return app.Apps{}, err
	}

	return apps, nil
}

func run(ctx context.Context, apps *app.Apps, size int, logger *slog.Logger) *server.Server {
	// run the server
	server := server.NewServer(apps, size, logger)
	server.Start(ctx)
	return server
}

func stop(cancel context.CancelFunc, server *server.Server) {
	server.Shutdown(cancel)
}
