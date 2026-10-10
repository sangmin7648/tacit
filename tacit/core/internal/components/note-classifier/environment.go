package noteclassifier

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// ClaudeAvailable reports whether the Claude Code CLI, which the claude
// provider shells out to, is on PATH.
func ClaudeAvailable() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// OllamaStatus is what could be learned about Ollama on this machine.
type OllamaStatus struct {
	// Installed is true when the CLI is on PATH or Ollama.app is installed.
	Installed bool `json:"installed"`
	// Running is true when the server answered; Models is only filled then.
	Running bool     `json:"running"`
	Models  []string `json:"models"`
}

// DetectOllama looks for Ollama without starting or changing anything. A
// server that does not answer within a couple of seconds counts as not running,
// so a stuck one cannot hold up the window that asks.
func DetectOllama(ctx context.Context) OllamaStatus {
	var st OllamaStatus
	if _, err := exec.LookPath("ollama"); err == nil {
		st.Installed = true
	} else if _, err := os.Stat("/Applications/Ollama.app"); err == nil {
		st.Installed = true
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if models, err := ollamaModels(ctx, &http.Client{}, defaultOllamaBaseURL); err == nil {
		st.Installed, st.Running, st.Models = true, true, models
	}
	return st
}

// PullOllamaModel downloads model into the local Ollama server, reporting
// bytes done out of total across all the layers seen so far. Cancelling ctx
// stops the pull; Ollama keeps the layers fetched, so a retry resumes.
func PullOllamaModel(ctx context.Context, model string, progress func(done, total int64)) error {
	payload, _ := json.Marshal(map[string]any{"model": model, "stream": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultOllamaBaseURL+"/api/pull", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("Ollama server not reachable at %s\n  → Is Ollama running? Try: ollama serve", defaultOllamaBaseURL)
	}
	defer resp.Body.Close()

	type layer struct{ done, total int64 }
	layers := map[string]layer{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var line struct {
			Error     string `json:"error"`
			Digest    string `json:"digest"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}
		if line.Error != "" {
			return errors.New(line.Error)
		}
		if line.Digest == "" || line.Total == 0 || progress == nil {
			continue
		}
		layers[line.Digest] = layer{line.Completed, line.Total}
		var done, total int64
		for _, l := range layers {
			done += l.done
			total += l.total
		}
		progress(done, total)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return sc.Err()
}
