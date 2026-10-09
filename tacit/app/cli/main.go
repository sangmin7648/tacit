package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sangmin7648/tacit/core/workflows/browse"
	"github.com/sangmin7648/tacit/core/workflows/configure"

	"golang.org/x/term"

	"github.com/sangmin7648/tacit/core/workflows/listen"
	"github.com/sangmin7648/tacit/core/workflows/onboard"
)

// version is set at build time by the Makefile (-X main.version).
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version", "--version":
		fmt.Printf("tacit %s\n", version)
	case "setup":
		// setup's writes are refused on a file that does not load; fail
		// before the wizard asks its questions rather than after.
		loadConfig()
		cmdSetup()
	case "listen":
		cmdListen(loadConfig())
	case "stop":
		cmdStop()
	case "status":
		cmdStatus()
	case "update":
		cmdUpdate()
	case "install-skills":
		if err := runInstallSkills(loadConfig().SkillAgent); err != nil {
			log.Fatalf("Failed to install skills: %v", err)
		}
		fmt.Println("Skills updated.")
	case "list":
		cmdList()
	case "search":
		cmdSearch()
	case "get":
		cmdGet()
	case "config":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: tacit config <view|edit|set|unset>\n")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "view":
			cmdConfigView(loadConfig())
		case "edit":
			cmdConfigEdit()
		case "set":
			cmdConfigSet()
		case "unset":
			cmdConfigUnset()
		default:
			fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", os.Args[2])
			fmt.Fprintf(os.Stderr, "Usage: tacit config <view|edit|set|unset>\n")
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// loadConfig loads the merged config, exiting if it cannot be loaded. Only
// commands that use the config call it, so a broken override file never blocks
// 'tacit config edit', the way to fix it.
func loadConfig() *configure.Settings {
	cfg, err := configure.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v\nFix it with 'tacit config edit'.", err)
	}
	return cfg
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `tacit - STT Knowledge DB

Usage:
  tacit setup                  Install Claude Code skill for knowledge base
  tacit listen                 Start the voice capture daemon (foreground)
  tacit stop                   Stop the voice capture daemon
  tacit status [--json]        Check daemon status
  tacit update                 Update tacit to the latest version
  tacit version                Print the installed version
  tacit list [duration] [--json]   List knowledge entries (default: 24h)
  tacit search [--duration <d>] [--json] <pattern>  Search knowledge entries by pattern
  tacit get [--json] <file-path>...  Print the full content of one or more knowledge entries
  tacit config view [--json]   Show current configuration
  tacit config edit            Open configuration in a text editor
  tacit config set <key> <value>  Override one setting
  tacit config unset <key>     Clear an override so the default applies
`)
}

// cmdSetup runs the interactive setup wizard. It only asks and reports: the
// decisions live in pkg/setup, which the Mac app's onboarding calls too.
func cmdSetup() {
	fmt.Println("=== tacit setup ===")
	fmt.Println()

	c := onboard.Defaults()

	// Step 1: LLM provider
	fmt.Println("Step 1/5: Select LLM provider for summarization")
	c.LLMProvider = onboard.Providers[selectOption(onboard.Providers, 0)]
	fmt.Println()

	switch c.LLMProvider {
	case "claude":
		// Step 2: Claude model
		fmt.Println("Step 2/5: Select Claude model")
		c.LLMModel = onboard.ClaudeModels[selectOption(onboard.ClaudeModels, 0)]
		fmt.Println()

	default:
		// Step 2: Ollama model (text input)
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Step 2/5: Enter Ollama model name")
		fmt.Printf("  Model name [%s]: ", onboard.DefaultOllamaModel)
		input := strings.TrimSpace(readLine(reader))
		fmt.Println()
		c.LLMModel = onboard.DefaultOllamaModel
		if input != "" {
			c.LLMModel = input
		}
	}

	// Step 3: AI agent for skill installation (only claude supported)
	fmt.Println("Step 3/5: Select AI agent for skill installation")
	c.SkillAgent = onboard.Agents[selectOption(onboard.Agents, 0)]
	fmt.Println()

	// Step 4: transcription language. Fixing the language (instead of "auto")
	// meaningfully reduces wrong-language / hallucinated transcriptions.
	fmt.Println("Step 4/5: Select transcription language")
	labels := make([]string, len(onboard.Languages))
	for i, l := range onboard.Languages {
		labels[i] = l.Label
	}
	c.Language = onboard.Languages[selectOption(labels, 0)].Code
	fmt.Println()

	// Step 5: experimental beta channel.
	fmt.Println("Step 5/5: Enable experimental transcription? (non-speech token suppression + VAD pre-roll padding)")
	c.Experimental = selectOption([]string{"no", "yes"}, 0) == 1
	fmt.Println()

	fmt.Println()
	fmt.Printf("  LLM provider   : %s\n", c.LLMProvider)
	fmt.Printf("  LLM model      : %s\n", c.LLMModel)
	fmt.Printf("  Skill agent    : %s\n", c.SkillAgent)
	fmt.Printf("  Language       : %s\n", c.Language)
	fmt.Printf("  Experimental   : %v\n", c.Experimental)
	fmt.Println()

	res, err := onboard.Apply(c)
	if err != nil {
		log.Fatalf("Setup failed: %v", err)
	}

	fmt.Printf("Saved settings: %s\n", res.OverridePath)
	for _, dest := range res.InstalledSkills {
		fmt.Printf("Installed: %s\n", dest)
	}
	if res.BackupPath != "" {
		fmt.Printf("WARNING: config.yaml appears to have been edited manually.\n")
		fmt.Printf("  tacit now uses config-override.yaml for user settings.\n")
		fmt.Printf("  Your previous config.yaml has been backed up to:\n")
		fmt.Printf("    %s\n", res.BackupPath)
		fmt.Printf("  Run 'tacit config edit' to set your overrides in config-override.yaml.\n\n")
	}
	fmt.Printf("Updated reference config: %s\n", res.ReferencePath)

	cfg := loadConfig()
	modelFile := filepath.Base(onboard.ModelPath(cfg))
	if err := onboard.DownloadModel(context.Background(), onboard.PrintProgress(modelFile)); err != nil {
		log.Fatalf("Downloading %s failed: %v", modelFile, err)
	}
	fmt.Println()

	fmt.Println("Setup complete.")
}

// readLine reads a single line from r, trimming the trailing newline.
func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

// selectOption presents an interactive arrow-key menu on stdout and returns the
// index of the selected option. defaultIdx is highlighted initially.
func selectOption(options []string, defaultIdx int) int {
	cur := defaultIdx

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback: print numbered list and read a line
		for i, o := range options {
			fmt.Printf("  %d) %s\n", i+1, o)
		}
		fmt.Printf("Choice [%d]: ", defaultIdx+1)
		reader := bufio.NewReader(os.Stdin)
		line := strings.TrimSpace(readLine(reader))
		for i := range options {
			if line == fmt.Sprintf("%d", i+1) {
				return i
			}
		}
		return defaultIdx
	}
	defer term.Restore(fd, oldState)

	// draw prints all options, then moves cursor back to top.
	// On the first call (atTop=true) cursor is already at top; subsequent
	// calls move up first so the list is redrawn in-place.
	draw := func(atTop bool) {
		if !atTop {
			fmt.Printf("\033[%dA", len(options))
		}
		for i, o := range options {
			fmt.Print("\r\033[2K") // carriage-return + erase line
			if i == cur {
				fmt.Printf("  \033[36m> %s\033[0m\n", o)
			} else {
				fmt.Printf("    %s\n", o)
			}
		}
	}

	draw(true)

	buf := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		switch {
		case n == 1 && (buf[0] == '\r' || buf[0] == '\n'): // Enter
			// Erase the list and print a single confirmation line.
			fmt.Printf("\033[%dA", len(options))
			for range options {
				fmt.Print("\r\033[2K\n")
			}
			fmt.Printf("\033[%dA", len(options))
			fmt.Printf("\r\033[2K  > %s\n", options[cur])
			return cur
		case n >= 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'A': // Up
			if cur > 0 {
				cur--
				draw(false)
			}
		case n >= 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'B': // Down
			if cur < len(options)-1 {
				cur++
				draw(false)
			}
		}
	}

	return cur
}

// cmdListen runs the daemon in the foreground until SIGINT or SIGTERM.
func cmdListen(cfg *configure.Settings) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Printf("Press Ctrl+C to stop")
	if err := listen.Run(ctx, cfg); err != nil {
		log.Fatalf("tacit listen: %v", err)
	}
}

// cmdStop sends SIGTERM to the running daemon.
func cmdStop() {
	pidPath := listen.PIDPath()
	running, pid := listen.Status(pidPath)
	if !running {
		listen.RemovePID(pidPath)
		fmt.Println("tacit is not running")
		return
	}
	if err := listen.Stop(pidPath); err != nil {
		log.Fatalf("Failed to stop tacit (PID %d): %v", pid, err)
	}
	fmt.Printf("Sent SIGTERM to tacit (PID: %d)\n", pid)
}

// cmdConfigView prints the current configuration, annotating each field as
// [default] or [override] based on whether it appears in config-override.yaml.
func cmdConfigView(cfg *configure.Settings) {
	cfgPath := configure.ReferencePath()
	overridePath := configure.OverridePath()

	if _, asJSON := stripFlag(os.Args[3:], "--json"); asJSON {
		fields, err := configure.Fields()
		if err != nil {
			log.Fatalf("Failed to read config: %v", err)
		}
		printJSON(configDoc{Version: jsonVersion, Reference: cfgPath, Override: overridePath, Fields: fields})
		return
	}

	fmt.Printf("Config files:\n")
	fmt.Printf("  reference: %s\n", cfgPath)
	fmt.Printf("  overrides: %s\n\n", overridePath)

	overrideKeys, err := configure.OverrideKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read override file: %v\n\n", err)
		overrideKeys = map[string]bool{}
	}

	tag := func(yamlKey string) string {
		if overrideKeys[yamlKey] {
			return "[override]"
		}
		return "[default]"
	}

	fmt.Printf("%-30s %-20s %s\n", "whisper_model:", cfg.WhisperModel, tag("whisper_model"))
	fmt.Printf("%-30s %-20s %s\n", "language:", cfg.Language, tag("language"))
	fmt.Printf("%-30s %-20v %s\n", "experimental:", cfg.Experimental, tag("experimental"))
	if cfg.InitialPrompt != "" {
		fmt.Printf("%-30s %-20s %s\n", "initial_prompt:", cfg.InitialPrompt, tag("initial_prompt"))
	}
	fmt.Printf("%-30s %-20s %s\n", "min_speech_duration:", cfg.MinSpeechDur, tag("min_speech_duration"))
	fmt.Printf("%-30s %-20s %s\n", "silence_duration:", cfg.SilenceDuration, tag("silence_duration"))
	fmt.Printf("%-30s %-20s %s\n", "max_segment_duration:", cfg.MaxSegmentDur, tag("max_segment_duration"))
	fmt.Printf("%-30s %-20s %s\n", "max_session_duration:", cfg.MaxSessionDur, tag("max_session_duration"))
	fmt.Printf("%-30s %-20s %s\n", "dedup_window:", cfg.DedupWindow, tag("dedup_window"))
	fmt.Printf("%-30s %-20.2f %s\n", "min_char_rate:", cfg.MinCharRate, tag("min_char_rate"))
	fmt.Printf("%-30s %-20.2f %s\n", "speech_threshold:", cfg.SpeechThreshold, tag("speech_threshold"))
	fmt.Printf("%-30s %-20.0f %s\n", "energy_threshold:", cfg.EnergyThreshold, tag("energy_threshold"))
	fmt.Printf("%-30s %-20s %s\n", "llm_provider:", cfg.LLMProvider, tag("llm_provider"))
	fmt.Printf("%-30s %-20s %s\n", "llm_model:", cfg.LLMModel, tag("llm_model"))
	fmt.Printf("%-30s %-20s %s\n", "skill_agent:", cfg.SkillAgent, tag("skill_agent"))
	if len(cfg.TranscriptDenylist) > 0 {
		fmt.Printf("%-30s %-20s %s\n", "transcript_denylist:", strings.Join(cfg.TranscriptDenylist, ", "), tag("transcript_denylist"))
	}
}

// cmdConfigEdit opens the user override config file in a text editor.
// It creates a commented template if the file does not exist yet.
func cmdConfigEdit() {
	overridePath := configure.OverridePath()

	created, err := configure.EnsureOverrideFile()
	if err != nil {
		log.Fatalf("Failed to create override template: %v", err)
	}
	if created {
		fmt.Printf("Created override template at %s\n", overridePath)
	}

	editor := detectEditor()
	if editor == "" {
		fmt.Fprintf(os.Stderr, "No text editor found. Set $EDITOR or install one of: nano, vim, vi, emacs.\n")
		fmt.Fprintf(os.Stderr, "Override config file is at: %s\n", overridePath)
		os.Exit(1)
	}

	cmd := exec.Command(editor, overridePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Editor exited with error: %v", err)
	}
}

// detectEditor returns the path to an available text editor.
// Priority: $VISUAL → $EDITOR → well-known editors in PATH.
func detectEditor() string {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if e := os.Getenv(env); e != "" {
			if path, err := exec.LookPath(e); err == nil {
				return path
			}
		}
	}

	candidates := []string{"nano", "vim", "vi", "emacs", "micro", "hx", "code", "subl", "gedit", "kate"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	return ""
}

// cmdUpdate updates tacit to the latest version by running the install script,
// then automatically installs the updated skills from the new binary.
func cmdUpdate() {
	sh, err := exec.LookPath("sh")
	if err != nil {
		log.Fatalf("sh not found: %v", err)
	}

	curl, err := exec.LookPath("curl")
	if err != nil {
		log.Fatalf("curl not found: %v", err)
	}

	fmt.Println("Updating tacit to the latest version...")

	cmd := exec.Command(sh, "-c", curl+" -fsSL https://raw.githubusercontent.com/sangmin7648/tacit/main/install.sh | "+sh)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Update failed: %v", err)
	}

	// Install skills from the newly downloaded binary so that skill changes
	// are applied without requiring a separate `tacit setup` run.
	newBin, err := exec.LookPath("tacit")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not find tacit binary to update skills: %v\n", err)
		return
	}
	skillCmd := exec.Command(newBin, "install-skills")
	skillCmd.Stdin = os.Stdin
	skillCmd.Stdout = os.Stdout
	skillCmd.Stderr = os.Stderr
	if err := skillCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: skill update failed: %v\n", err)
	}
}

// runInstallSkills installs the embedded skill files for agent and prints
// each destination path written.
func runInstallSkills(agent string) error {
	installed, err := onboard.InstallSkills(agent)
	for _, dest := range installed {
		fmt.Printf("Installed: %s\n", dest)
	}
	return err
}

// parseDuration extends time.ParseDuration with support for d (days) and w (weeks).
func parseDuration(s string) (time.Duration, error) {
	// Replace w and d with their hour equivalents before parsing.
	// Process longest suffixes first to avoid partial replacement.
	result := time.Duration(0)
	remaining := s
	for remaining != "" {
		// Find next numeric run
		i := 0
		for i < len(remaining) && (remaining[i] >= '0' && remaining[i] <= '9') {
			i++
		}
		if i == 0 {
			// Non-numeric start — pass the whole thing to time.ParseDuration for error
			return time.ParseDuration(s)
		}
		numStr := remaining[:i]
		remaining = remaining[i:]

		// Find the unit (non-numeric, non-dot characters)
		j := 0
		for j < len(remaining) && !(remaining[j] >= '0' && remaining[j] <= '9') {
			j++
		}
		unit := remaining[:j]
		remaining = remaining[j:]

		var n int64
		fmt.Sscanf(numStr, "%d", &n)

		switch unit {
		case "d":
			result += time.Duration(n) * 24 * time.Hour
		case "w":
			result += time.Duration(n) * 7 * 24 * time.Hour
		default:
			// Re-parse this token with standard parser
			d, err := time.ParseDuration(numStr + unit)
			if err != nil {
				return 0, fmt.Errorf("unknown unit %q in duration %q", unit, s)
			}
			result += d
		}
	}
	return result, nil
}

// cmdList lists knowledge entries created within the given duration (default 24h).
func cmdList() {
	args, asJSON := stripFlag(os.Args[2:], "--json")
	dur := 24 * time.Hour
	if len(args) >= 1 {
		d, err := parseDuration(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid duration %q: %v\n", args[0], err)
			fmt.Fprintf(os.Stderr, "Examples: 1h, 30m, 24h, 1d, 7d, 2w\n")
			os.Exit(1)
		}
		dur = d
	}

	cutoff := time.Now().Add(-dur)

	entries, err := browse.List(cutoff)
	if err != nil {
		log.Fatalf("Failed to read knowledge base: %v", err)
	}

	if asJSON {
		printJSON(listDoc{Version: jsonVersion, Since: cutoff, Entries: normalizeEntries(entries)})
		return
	}

	durStr := formatDuration(dur)

	if len(entries) == 0 {
		fmt.Printf("No entries found in the last %s.\n", durStr)
		return
	}

	fmt.Printf("Found %d entries in the last %s:\n\n", len(entries), durStr)
	for _, e := range entries {
		fmt.Printf("[%s] %s / %s\n", e.CreatedAt.Format("2006-01-02 15:04:05"), e.Category, e.Title)
		fmt.Printf("  File:    %s\n", e.FilePath)
		if e.Summary != "" {
			// Print first line of summary
			summary := e.Summary
			if idx := findNewline(summary); idx >= 0 {
				summary = summary[:idx]
			}
			fmt.Printf("  Summary: %s\n", summary)
		}
		fmt.Println()
	}
}

// cmdSearch searches the knowledge base for entries matching a pattern.
func cmdSearch() {
	// Parse args: tacit search [--duration <dur>] [--json] <pattern>
	args, asJSON := stripFlag(os.Args[2:], "--json")
	var since time.Time
	var patternArgs []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--duration" && i+1 < len(args) {
			d, err := parseDuration(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid duration %q: %v\n", args[i+1], err)
				fmt.Fprintf(os.Stderr, "Examples: 1h, 30m, 24h, 1d, 7d, 2w\n")
				os.Exit(1)
			}
			since = time.Now().Add(-d)
			i++ // skip duration value
		} else {
			patternArgs = append(patternArgs, args[i])
		}
	}

	if len(patternArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: tacit search [--duration <duration>] <pattern>\n")
		fmt.Fprintf(os.Stderr, "Examples: tacit search meeting, tacit search --duration 1h meeting\n")
		os.Exit(1)
	}

	pattern := patternArgs[0]
	results, err := browse.Search(pattern, since)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	if asJSON {
		doc := searchDoc{Version: jsonVersion, Pattern: pattern, Results: results}
		if !since.IsZero() {
			doc.Since = &since
		}
		if doc.Results == nil {
			doc.Results = []*browse.SearchResult{}
		}
		for _, r := range doc.Results {
			if r.KnowledgeEntry != nil && r.Keywords == nil {
				r.Keywords = []string{}
			}
			if r.MatchLines == nil {
				r.MatchLines = []string{}
			}
		}
		printJSON(doc)
		return
	}

	if len(results) == 0 {
		if !since.IsZero() {
			fmt.Printf("No results found for %q in the last %s.\n", pattern, formatDuration(time.Since(since)))
		} else {
			fmt.Printf("No results found for %q.\n", pattern)
		}
		return
	}

	if !since.IsZero() {
		fmt.Printf("Found %d result(s) for %q in the last %s:\n\n", len(results), pattern, formatDuration(time.Since(since)))
	} else {
		fmt.Printf("Found %d result(s) for %q:\n\n", len(results), pattern)
	}
	for _, r := range results {
		fmt.Printf("[%s] %s / %s\n", r.CreatedAt.Format("2006-01-02 15:04:05"), r.Category, r.Title)
		fmt.Printf("  File:  %s\n", r.FilePath)
		for _, line := range r.MatchLines {
			fmt.Printf("  Match: %s\n", line)
		}
		fmt.Println()
	}
}

// cmdGet prints the full content of one or more knowledge entry files.
func cmdGet() {
	filePaths, asJSON := stripFlag(os.Args[2:], "--json")
	if len(filePaths) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: tacit get [--json] <file-path> [<file-path>...]\n")
		os.Exit(1)
	}

	if asJSON {
		// A failed read is reported in-band and does not change the exit code,
		// matching the text output, which notes it on stderr and moves on.
		doc := getDoc{Version: jsonVersion, Entries: []*browse.Note{}, Errors: []getError{}}
		for _, filePath := range filePaths {
			entry, err := browse.Read(filePath)
			if err != nil {
				doc.Errors = append(doc.Errors, getError{Path: filePath, Error: err.Error()})
				continue
			}
			doc.Entries = append(doc.Entries, entry)
		}
		doc.Entries = normalizeEntries(doc.Entries)
		printJSON(doc)
		return
	}

	for i, filePath := range filePaths {
		if i > 0 {
			fmt.Println()
			fmt.Println("---")
			fmt.Println()
		}
		entry, err := browse.Read(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read entry %s: %v\n", filePath, err)
			continue
		}

		fmt.Printf("Title:    %s\n", entry.Title)
		fmt.Printf("Category: %s\n", entry.Category)
		fmt.Printf("Created:  %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("File:     %s\n", entry.FilePath)
		fmt.Println()
		if entry.Summary != "" {
			fmt.Println("## Summary")
			fmt.Println()
			fmt.Println(entry.Summary)
			fmt.Println()
		}
		if entry.Content != "" {
			fmt.Println("## Content")
			fmt.Println()
			fmt.Println(entry.Content)
		}
	}
}

// formatDuration formats a duration using d/w units when possible.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	weeks := int(d / (7 * 24 * time.Hour))
	rem := d % (7 * 24 * time.Hour)
	days := int(rem / (24 * time.Hour))
	rem = rem % (24 * time.Hour)

	if rem == 0 {
		if weeks > 0 && days == 0 {
			return fmt.Sprintf("%dw", weeks)
		}
		totalDays := weeks*7 + days
		if totalDays > 0 {
			return fmt.Sprintf("%dd", totalDays)
		}
	}
	// Fall back to standard format for sub-day durations or mixed units
	return d.String()
}

func findNewline(s string) int {
	for i, c := range s {
		if c == '\n' {
			return i
		}
	}
	return -1
}

// cmdStatus checks if the daemon is running.
func cmdStatus() {
	_, asJSON := stripFlag(os.Args[2:], "--json")
	pidPath := listen.PIDPath()

	pid, err := listen.ReadPID(pidPath)
	if err != nil {
		if asJSON {
			printJSON(statusDoc{Version: jsonVersion})
			return
		}
		fmt.Println("tacit is not running")
		return
	}

	if asJSON {
		doc := statusDoc{Version: jsonVersion}
		if listen.IsRunning(pid) {
			doc.Running = true
			doc.PID = pid
			if info, err := os.Stat(pidPath); err == nil {
				t := info.ModTime()
				doc.StartedAt = &t
			}
		} else {
			listen.RemovePID(pidPath)
		}
		printJSON(doc)
		return
	}

	if listen.IsRunning(pid) {
		fmt.Printf("tacit is running (PID: %d)\n", pid)
	} else {
		listen.RemovePID(pidPath)
		fmt.Println("tacit is not running (stale PID cleaned)")
	}
}
