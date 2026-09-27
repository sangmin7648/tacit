package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sangmin7648/tacit/pkg/capture"
	"github.com/sangmin7648/tacit/pkg/config"
	"github.com/sangmin7648/tacit/pkg/daemon"
	"github.com/sangmin7648/tacit/pkg/events"
	"github.com/sangmin7648/tacit/pkg/pipeline"
	"github.com/sangmin7648/tacit/pkg/search"
	"github.com/sangmin7648/tacit/pkg/setup"
	"github.com/sangmin7648/tacit/pkg/storage"
	"github.com/sangmin7648/tacit/skills"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		// setup's writes are refused on a file that does not load; fail
		// before the wizard asks its questions rather than after.
		loadConfig()
		cmdSetup()
	case "process":
		cmdProcess(loadConfig())
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
func loadConfig() *config.Config {
	cfg, err := config.LoadWithOverride(config.ConfigPath(), config.OverridePath())
	if err != nil {
		log.Fatalf("Failed to load config: %v\nFix it with 'tacit config edit'.", err)
	}
	return cfg
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `tacit - STT Knowledge DB

Usage:
  tacit setup                  Install Claude Code skill for knowledge base
  tacit process <audio-file>   Process an audio file into a knowledge entry
  tacit listen                 Start the voice capture daemon (foreground)
  tacit stop                   Stop the voice capture daemon
  tacit status [--json]        Check daemon status
  tacit update                 Update tacit to the latest version
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

	c := setup.Defaults()

	// Step 1: LLM provider
	fmt.Println("Step 1/6: Select LLM provider for summarization")
	c.LLMProvider = setup.Providers[selectOption(setup.Providers, 0)]
	fmt.Println()

	switch c.LLMProvider {
	case "claude":
		// Step 2: Claude model
		fmt.Println("Step 2/6: Select Claude model")
		c.LLMModel = setup.ClaudeModels[selectOption(setup.ClaudeModels, 0)]
		fmt.Println()

	default:
		// Step 2: Ollama model (text input)
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Step 2/6: Enter Ollama model name")
		fmt.Printf("  Model name [%s]: ", setup.DefaultOllamaModel)
		input := strings.TrimSpace(readLine(reader))
		fmt.Println()
		c.LLMModel = setup.DefaultOllamaModel
		if input != "" {
			c.LLMModel = input
		}
	}

	// Step 3: AI agent for skill installation (only claude supported)
	fmt.Println("Step 3/6: Select AI agent for skill installation")
	c.SkillAgent = setup.Agents[selectOption(setup.Agents, 0)]
	fmt.Println()

	// Step 4: Audio sources (multi-select) — at least one must be selected.
	for {
		fmt.Println("Step 4/6: Select audio sources to listen  (Space to toggle, Enter to confirm)")
		sourceSelected := selectMultiple([]string{"mic", "speaker"}, []bool{true, true})
		fmt.Println()
		c.CaptureMic, c.CaptureSpeaker = sourceSelected[0], sourceSelected[1]
		if c.CaptureMic || c.CaptureSpeaker {
			break
		}
		fmt.Println("  At least one source must be selected. Please try again.")
		fmt.Println()
	}

	// Step 5: transcription language. Fixing the language (instead of "auto")
	// meaningfully reduces wrong-language / hallucinated transcriptions.
	fmt.Println("Step 5/6: Select transcription language")
	labels := make([]string, len(setup.Languages))
	for i, l := range setup.Languages {
		labels[i] = l.Label
	}
	c.Language = setup.Languages[selectOption(labels, 0)].Code
	fmt.Println()

	// Step 6: experimental beta channel.
	fmt.Println("Step 6/6: Enable experimental transcription? (non-speech token suppression + VAD pre-roll padding)")
	c.Experimental = selectOption([]string{"no", "yes"}, 0) == 1
	fmt.Println()

	fmt.Println()
	fmt.Printf("  LLM provider   : %s\n", c.LLMProvider)
	fmt.Printf("  LLM model      : %s\n", c.LLMModel)
	fmt.Printf("  Skill agent    : %s\n", c.SkillAgent)
	fmt.Printf("  Capture mic    : %v\n", c.CaptureMic)
	fmt.Printf("  Capture speaker: %v\n", c.CaptureSpeaker)
	fmt.Printf("  Language       : %s\n", c.Language)
	fmt.Printf("  Experimental   : %v\n", c.Experimental)
	fmt.Println()

	res, err := setup.Apply(c)
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

// selectMultiple presents an interactive checkbox menu on stdout. Arrow keys
// move the cursor; Space toggles the current item; Enter confirms. Returns a
// slice of booleans aligned with options indicating the selected state.
// defaultSelected sets the initial checked state for each option.
func selectMultiple(options []string, defaultSelected []bool) []bool {
	selected := make([]bool, len(options))
	copy(selected, defaultSelected)
	cur := 0

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback: numbered list
		for i, o := range options {
			mark := " "
			if selected[i] {
				mark = "x"
			}
			fmt.Printf("  [%s] %d) %s\n", mark, i+1, o)
		}
		fmt.Print("Toggle items by number (space-separated), then press Enter: ")
		reader := bufio.NewReader(os.Stdin)
		line := strings.TrimSpace(readLine(reader))
		for _, tok := range strings.Fields(line) {
			for i := range options {
				if tok == fmt.Sprintf("%d", i+1) {
					selected[i] = !selected[i]
				}
			}
		}
		return selected
	}
	defer term.Restore(fd, oldState)

	draw := func(atTop bool) {
		if !atTop {
			fmt.Printf("\033[%dA", len(options))
		}
		for i, o := range options {
			fmt.Print("\r\033[2K")
			mark := " "
			if selected[i] {
				mark = "x"
			}
			if i == cur {
				fmt.Printf("  \033[36m> [%s] %s\033[0m\n", mark, o)
			} else {
				fmt.Printf("    [%s] %s\n", mark, o)
			}
		}
	}

	draw(true)

	buf := make([]byte, 4)
	for {
		n, readErr := os.Stdin.Read(buf)
		if readErr != nil || n == 0 {
			break
		}
		switch {
		case n == 1 && (buf[0] == '\r' || buf[0] == '\n'): // Enter — confirm
			fmt.Printf("\033[%dA", len(options))
			for range options {
				fmt.Print("\r\033[2K\n")
			}
			fmt.Printf("\033[%dA", len(options))
			for i, o := range options {
				mark := " "
				if selected[i] {
					mark = "x"
				}
				fmt.Printf("\r\033[2K  [%s] %s\n", mark, o)
			}
			return selected
		case n == 1 && buf[0] == ' ': // Space — toggle
			selected[cur] = !selected[cur]
			draw(false)
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

	return selected
}

// cmdProcess handles the "process" subcommand: audio file → knowledge entry.
// openEventLog attaches the daemon event log to p, so a front end can follow
// this run. A failure here is reported and swallowed: the event log is an
// observation channel, and losing it must never cost the user a transcript.
// The returned closer is nil when the log could not be opened.
func openEventLog(p *pipeline.Pipeline) func() {
	w, err := events.NewWriter(config.EventLogPath(), 0)
	if err != nil {
		log.Printf("Warning: event log unavailable: %v", err)
		return nil
	}
	p.SetObserver(w)
	return func() {
		if err := w.Err(); err != nil {
			log.Printf("Warning: event log write failed: %v", err)
		}
		w.Close()
	}
}

func cmdProcess(cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: tacit process <audio-file>\n")
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

	closeEvents := openEventLog(p)

	filePath, err := p.ProcessFile(ctx, audioPath)
	p.Close() // Close before printing to avoid ggml cleanup race
	// Closed explicitly rather than deferred: every exit below is an os.Exit or
	// log.Fatalf, and neither runs deferred functions.
	if closeEvents != nil {
		closeEvents()
	}

	if err != nil {
		if errors.Is(err, pipeline.ErrSkipped) {
			fmt.Println("Content classified as meaningless, skipping.")
			os.Exit(0)
		}
		log.Fatalf("Processing failed: %v", err)
	}

	fmt.Printf("Knowledge entry created: %s\n", filePath)
	os.Exit(0) // Exit immediately to avoid ggml Metal cleanup crash
}

// cmdListen starts the voice capture pipeline in the foreground.
func cmdListen(cfg *config.Config) {
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

	if closeEvents := openEventLog(p); closeEvents != nil {
		defer closeEvents()
	}

	// Build audio sources.
	var sources []capture.AudioSource
	var sourceLabels []string

	if cfg.CaptureMic {
		mic, err := capture.New()
		if err != nil {
			log.Fatalf("Failed to init microphone: %v", err)
		}
		defer mic.Close()
		sources = append(sources, mic)
		sourceLabels = append(sourceLabels, "mic")
	}

	if cfg.CaptureSpeaker {
		spk, err := capture.NewSpeaker()
		if err != nil {
			log.Printf("Warning: system audio capture unavailable: %v", err)
			if len(sources) == 0 {
				log.Fatalf("No audio sources available.")
			}
			log.Printf("Continuing with microphone only.")
		} else {
			defer spk.Close()
			sources = append(sources, spk)
			sourceLabels = append(sourceLabels, "speaker")
			log.Printf("System audio capture enabled (requires Screen Recording permission)")
		}
	}

	if len(sources) == 0 {
		log.Fatalf("No audio sources configured. Enable capture_mic or capture_speaker in config.")
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Printf("tacit daemon started (PID: %d)", os.Getpid())
	log.Printf("Knowledge base: %s", config.BaseDir())
	log.Printf("Press Ctrl+C to stop")

	if err := p.Run(ctx, sources, sourceLabels); err != nil {
		log.Printf("Pipeline error: %v", err)
	}
	log.Printf("tacit daemon stopped")
}

// cmdStop sends SIGTERM to the running daemon.
func cmdStop() {
	pidPath := config.PIDPath()

	pid, err := daemon.ReadPID(pidPath)
	if err != nil {
		fmt.Println("tacit is not running")
		return
	}

	if !daemon.IsRunning(pid) {
		daemon.RemovePID(pidPath)
		fmt.Println("tacit is not running (stale PID cleaned)")
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		log.Fatalf("Failed to find process %d: %v", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		log.Fatalf("Failed to send SIGTERM to %d: %v", pid, err)
	}

	fmt.Printf("Sent SIGTERM to tacit (PID: %d)\n", pid)
}

// cmdConfigView prints the current configuration, annotating each field as
// [default] or [override] based on whether it appears in config-override.yaml.
func cmdConfigView(cfg *config.Config) {
	cfgPath := config.ConfigPath()
	overridePath := config.OverridePath()

	if _, asJSON := stripFlag(os.Args[3:], "--json"); asJSON {
		fields, err := config.Fields(cfgPath, overridePath)
		if err != nil {
			log.Fatalf("Failed to read config: %v", err)
		}
		printJSON(configDoc{Version: jsonVersion, Reference: cfgPath, Override: overridePath, Fields: fields})
		return
	}

	fmt.Printf("Config files:\n")
	fmt.Printf("  reference: %s\n", cfgPath)
	fmt.Printf("  overrides: %s\n\n", overridePath)

	overrideKeys, err := config.LoadOverrideKeys(overridePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read override file: %v\n\n", err)
		overrideKeys = map[string]bool{}
	}

	tag := func(yamlKey string) string {
		if overrideKeys[yamlKey] {
			return "[override]"
		}
		// An overridden shared timing applies to a source that does not set
		// its own (see config.LoadWithOverride).
		for _, prefix := range []string{"mic_", "speaker_"} {
			if shared, ok := strings.CutPrefix(yamlKey, prefix); ok && overrideKeys[shared] {
				return "[from " + shared + "]"
			}
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
	fmt.Printf("%-30s %-20s %s\n", "mic_min_speech_duration:", cfg.MicMinSpeechDur, tag("mic_min_speech_duration"))
	fmt.Printf("%-30s %-20s %s\n", "mic_silence_duration:", cfg.MicSilenceDuration, tag("mic_silence_duration"))
	fmt.Printf("%-30s %-20s %s\n", "mic_max_segment_duration:", cfg.MicMaxSegmentDur, tag("mic_max_segment_duration"))
	fmt.Printf("%-30s %-20s %s\n", "speaker_min_speech_duration:", cfg.SpeakerMinSpeechDur, tag("speaker_min_speech_duration"))
	fmt.Printf("%-30s %-20s %s\n", "speaker_silence_duration:", cfg.SpeakerSilenceDuration, tag("speaker_silence_duration"))
	fmt.Printf("%-30s %-20s %s\n", "speaker_max_segment_duration:", cfg.SpeakerMaxSegmentDur, tag("speaker_max_segment_duration"))
	fmt.Printf("%-30s %-20.2f %s\n", "speech_threshold:", cfg.SpeechThreshold, tag("speech_threshold"))
	fmt.Printf("%-30s %-20.0f %s\n", "energy_threshold:", cfg.EnergyThreshold, tag("energy_threshold"))
	fmt.Printf("%-30s %-20s %s\n", "llm_provider:", cfg.LLMProvider, tag("llm_provider"))
	fmt.Printf("%-30s %-20s %s\n", "llm_model:", cfg.LLMModel, tag("llm_model"))
	fmt.Printf("%-30s %-20s %s\n", "skill_agent:", cfg.SkillAgent, tag("skill_agent"))
	fmt.Printf("%-30s %-20v %s\n", "capture_mic:", cfg.CaptureMic, tag("capture_mic"))
	fmt.Printf("%-30s %-20v %s\n", "capture_speaker:", cfg.CaptureSpeaker, tag("capture_speaker"))
	if len(cfg.TranscriptDenylist) > 0 {
		fmt.Printf("%-30s %-20s %s\n", "transcript_denylist:", strings.Join(cfg.TranscriptDenylist, ", "), tag("transcript_denylist"))
	}
}

// cmdConfigEdit opens the user override config file in a text editor.
// It creates a commented template if the file does not exist yet.
func cmdConfigEdit() {
	overridePath := config.OverridePath()

	if _, err := os.Stat(overridePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(overridePath), 0755); err != nil {
			log.Fatalf("Failed to create config directory: %v", err)
		}
		if err := config.WriteOverrideTemplate(overridePath, config.DefaultConfig()); err != nil {
			log.Fatalf("Failed to create override template: %v", err)
		}
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
	installed, err := skills.Install(agent)
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

	baseDir := config.BaseDir()
	cutoff := time.Now().Add(-dur)

	entries, err := storage.ListEntries(baseDir, cutoff)
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
	baseDir := config.BaseDir()

	results, err := search.Search(baseDir, pattern, since)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	if asJSON {
		doc := searchDoc{Version: jsonVersion, Pattern: pattern, Results: results}
		if !since.IsZero() {
			doc.Since = &since
		}
		if doc.Results == nil {
			doc.Results = []*search.SearchResult{}
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
		doc := getDoc{Version: jsonVersion, Entries: []*storage.KnowledgeEntry{}, Errors: []getError{}}
		for _, filePath := range filePaths {
			entry, err := storage.Read(filePath)
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
		entry, err := storage.Read(filePath)
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
	pidPath := config.PIDPath()

	pid, err := daemon.ReadPID(pidPath)
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
		if daemon.IsRunning(pid) {
			doc.Running = true
			doc.PID = pid
			if info, err := os.Stat(pidPath); err == nil {
				t := info.ModTime()
				doc.StartedAt = &t
			}
		} else {
			daemon.RemovePID(pidPath)
		}
		printJSON(doc)
		return
	}

	if daemon.IsRunning(pid) {
		fmt.Printf("tacit is running (PID: %d)\n", pid)
	} else {
		daemon.RemovePID(pidPath)
		fmt.Println("tacit is not running (stale PID cleaned)")
	}
}
