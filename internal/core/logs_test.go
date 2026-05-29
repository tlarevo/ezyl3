package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileLogReaderReadsServiceLog(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.Mkdir(filepath.Join(runtime.Path, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime.Path, "logs", "litellm.out.log"), []byte("ready\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := FileLogReader{Runtime: runtime}.Read("litellm")
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if string(out) != "ready\n" {
		t.Fatalf("Read = %q", out)
	}
}

func TestFileLogReaderRejectsUnknownService(t *testing.T) {
	_, err := FileLogReader{Runtime: NewRuntime(t.TempDir())}.Read("other")
	if err == nil || !strings.Contains(err.Error(), "unknown log target") {
		t.Fatalf("Read error = %v", err)
	}
}

func TestFileLogReaderFollowStopsWhenContextIsCancelled(t *testing.T) {
	runtime := NewRuntime(t.TempDir())
	if err := os.Mkdir(filepath.Join(runtime.Path, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(runtime.Path, "logs", "litellm.out.log")
	if err := os.WriteFile(logPath, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- FileLogReader{Runtime: runtime, PollInterval: 10 * time.Millisecond}.Follow(ctx, "litellm", &out)
	}()
	time.Sleep(30 * time.Millisecond)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("second\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Follow returned error: %v", err)
	}
	if !strings.Contains(out.String(), "first\n") || !strings.Contains(out.String(), "second\n") {
		t.Fatalf("follow output = %q", out.String())
	}
}
