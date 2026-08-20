package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/G1DO/seshatops/forecast"
)

const (
	defaultCandidateArtifactTimeout = 30 * time.Second
	defaultCandidateArtifactOutput  = 1 << 20
)

// CandidateArtifactProducer is the bounded process boundary for the offline
// Python artifact producer. It receives only typed, read-only M4 input and
// returns the producer's JSON artifact for Go-owned validation.
type CandidateArtifactProducer interface {
	Produce(ctx context.Context, input forecast.CandidateInput) ([]byte, error)
}

// CommandCandidateArtifactProducer runs stockout_candidate.py without a shell.
// The input is written to a private temporary file because the existing Python
// producer's public CLI accepts an input path and writes its artifact to stdout.
type CommandCandidateArtifactProducer struct {
	Command        string
	Script         string
	Timeout        time.Duration
	MaxOutputBytes int
}

// Produce implements CandidateArtifactProducer with a deadline, bounded
// stdout, and a minimal environment. Python has no database or write path.
func (p CommandCandidateArtifactProducer) Produce(ctx context.Context, input forecast.CandidateInput) ([]byte, error) {
	if p.Command == "" || p.Script == "" {
		return nil, ErrPythonUnavailable
	}
	rawInput, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("platform: marshal candidate input: %w", err)
	}
	inputFile, err := os.CreateTemp("", "seshatops-forecast-input-*.json")
	if err != nil {
		return nil, fmt.Errorf("platform: create candidate input: %w", err)
	}
	inputPath := inputFile.Name()
	defer func() { _ = os.Remove(inputPath) }()
	if err := inputFile.Chmod(0o600); err != nil {
		_ = inputFile.Close()
		return nil, fmt.Errorf("platform: protect candidate input: %w", err)
	}
	if _, err := inputFile.Write(rawInput); err != nil {
		_ = inputFile.Close()
		return nil, fmt.Errorf("platform: write candidate input: %w", err)
	}
	if err := inputFile.Close(); err != nil {
		return nil, fmt.Errorf("platform: close candidate input: %w", err)
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultCandidateArtifactTimeout
	}
	maxOutput := p.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = defaultCandidateArtifactOutput
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, p.Command, p.Script, "--input", inputPath)
	output := limitedBuffer{limit: maxOutput}
	cmd.Stdout = &output
	cmd.Stderr = discardWriter{}
	cmd.Env = minimalPythonEnvironment()
	if err := cmd.Run(); err != nil {
		if errors.Is(output.err, errOutputLimit) {
			return nil, fmt.Errorf("%w: output exceeds limit", ErrPythonInvalidResponse)
		}
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return nil, ErrPythonTimeout
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrPythonUnavailable
	}
	if errors.Is(output.err, errOutputLimit) {
		return nil, fmt.Errorf("%w: output exceeds limit", ErrPythonInvalidResponse)
	}
	return append([]byte(nil), output.buf.Bytes()...), nil
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
