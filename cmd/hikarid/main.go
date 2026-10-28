package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"hikari/internal/api"
	"hikari/internal/config"
	"hikari/internal/logging"
)

var (
	version = "0.8.4"
	commit  = ""
)

func versionLine() string {
	if commit == "" {
		return version
	}
	return version + " (" + shortCommit(commit) + ")"
}

func shortCommit(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func main() {
	socket := flag.String("socket", "", "socket path (default $XDG_RUNTIME_DIR/hikari.sock)")
	verbose := flag.Bool("v", false, "debug logging")
	demo := flag.Bool("demo", false, "fake in-memory library, simulated installs and launches (UI development)")
	noWS := flag.Bool("no-ws", false, "do not serve the local websocket the browser UI uses")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("hikarid", versionLine())
		return
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	paths := config.Resolve()
	ring := logging.NewRing(500)
	var sink io.Writer = io.MultiWriter(os.Stderr, ring)
	if w, err := logging.NewRotatingWriter(paths.Log, 4<<20); err == nil {
		sink = io.MultiWriter(os.Stderr, ring, w)
		defer w.Close()
	} else {
		fmt.Fprintln(os.Stderr, "hikarid: cannot open the log file:", err)
	}
	log := slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	path := *socket
	if path == "" {
		path = config.SocketPath()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	d, err := newDaemon(log, *demo)
	if err != nil {
		log.Error("hikarid cannot start", "err", err)
		os.Exit(1)
	}
	d.logRing = ring
	d.logPath = paths.Log
	d.version = version
	d.commit = commit
	srv := api.NewServer(log)
	d.register(srv, ctx)

	if !*noWS {
		ep, err := startWS(ctx, log, srv, d.art)
		if err != nil {
			log.Error("hikarid cannot serve the local websocket", "err", err)
			os.Exit(1)
		}
		defer removeEndpoint()
		log.Info("hikarid websocket ready", "url", ep.URL, "token_file", config.WSEndpointPath())
	}

	log.Info("hikarid starting", "version", version, "commit", commit, "socket", path)
	if err := srv.ListenAndServe(ctx, path); err != nil {
		log.Error("daemon stopped", "err", err)
		os.Exit(1)
	}
}
