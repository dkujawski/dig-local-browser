package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunScanReportsStartAndCompletion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ordinary.bin"), []byte("ordinary"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "findings.jsonl")
	var stderr bytes.Buffer

	exitCode := Run(
		context.Background(),
		[]string{"scan", "--root", root, "--output", output, "--progress-interval", "0"},
		&bytes.Buffer{},
		&stderr,
	)

	if exitCode != ExitSuccess {
		t.Fatalf("Run() exit code=%d stderr=%q", exitCode, stderr.String())
	}
	for _, want := range []string{"INFO scan started", "INFO scan complete", "scanned=1"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr=%q; want %q", stderr.String(), want)
		}
	}
	if strings.Contains(stderr.String(), "INFO scan progress") {
		t.Errorf("stderr=%q; periodic progress should be disabled", stderr.String())
	}
}

func TestRunScanReportsPeriodicProgress(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ordinary.bin"), []byte("ordinary"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "findings.jsonl")
	var stderr bytes.Buffer

	exitCode := Run(
		context.Background(),
		[]string{"scan", "--root", root, "--output", output, "--progress-interval", "1ns"},
		&bytes.Buffer{},
		&stderr,
	)

	if exitCode != ExitSuccess {
		t.Fatalf("Run() exit code=%d stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "INFO scan progress") {
		t.Fatalf("stderr=%q; want periodic progress", stderr.String())
	}
}
