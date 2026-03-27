package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/rapportlabs/sttdb/pkg/config"
	"github.com/rapportlabs/sttdb/pkg/daemon"
	"github.com/rapportlabs/sttdb/pkg/pipeline"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	switch os.Args[1] {
	case "process":
		cmdProcess(cfg)
	case "start":
		cmdStart(cfg)
	case "stop":
		cmdStop()
	case "status":
		cmdStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `sttdb - STT Knowledge DB

Usage:
  sttdb process <audio-file>   Process an audio file into a knowledge entry
  sttdb start                  Start the voice capture daemon (foreground)
  sttdb stop                   Stop the voice capture daemon
  sttdb status                 Check daemon status
`)
}

// cmdProcess handles the "process" subcommand: audio file → knowledge entry.
func cmdProcess(cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: sttdb process <audio-file>\n")
		os.Exit(1)
	}

	audioPath := os.Args[2]
	if _, err := os.Stat(audioPath); err != nil {
		log.Fatalf("Audio file not found: %s", audioPath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := pipeline.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize pipeline: %v", err)
	}

	filePath, err := p.ProcessFile(ctx, audioPath)
	p.Close() // Close before printing to avoid ggml cleanup race

	if err != nil {
		log.Fatalf("Processing failed: %v", err)
	}

	fmt.Printf("Knowledge entry created: %s\n", filePath)
	os.Exit(0) // Exit immediately to avoid ggml Metal cleanup crash
}

// cmdStart starts the voice capture pipeline in the foreground.
func cmdStart(cfg *config.Config) {
	pidPath := config.PIDPath()

	// Check for existing daemon
	if err := daemon.CleanStalePID(pidPath); err != nil {
		log.Fatalf("Daemon already running: %v", err)
	}

	// Write PID
	if err := daemon.WritePID(pidPath); err != nil {
		log.Fatalf("Failed to write PID file: %v", err)
	}
	defer daemon.RemovePID(pidPath)

	// Create pipeline
	p, err := pipeline.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize pipeline: %v", err)
	}
	defer p.Close()

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Printf("sttdb daemon started (PID: %d)", os.Getpid())
	log.Printf("Knowledge base: %s", config.BaseDir())
	log.Printf("Press Ctrl+C to stop")

	// TODO: Start real-time microphone capture pipeline
	// For now, just wait for shutdown signal
	<-ctx.Done()
	log.Printf("sttdb daemon stopped")
}

// cmdStop sends SIGTERM to the running daemon.
func cmdStop() {
	pidPath := config.PIDPath()

	pid, err := daemon.ReadPID(pidPath)
	if err != nil {
		fmt.Println("sttdb is not running")
		return
	}

	if !daemon.IsRunning(pid) {
		daemon.RemovePID(pidPath)
		fmt.Println("sttdb is not running (stale PID cleaned)")
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("Failed to find process %d: %v", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		log.Fatalf("Failed to send SIGTERM to %d: %v", pid, err)
	}

	fmt.Printf("Sent SIGTERM to sttdb (PID: %d)\n", pid)
}

// cmdStatus checks if the daemon is running.
func cmdStatus() {
	pidPath := config.PIDPath()

	pid, err := daemon.ReadPID(pidPath)
	if err != nil {
		fmt.Println("sttdb is not running")
		return
	}

	if daemon.IsRunning(pid) {
		fmt.Printf("sttdb is running (PID: %s)\n", strconv.Itoa(pid))
	} else {
		daemon.RemovePID(pidPath)
		fmt.Println("sttdb is not running (stale PID cleaned)")
	}
}
