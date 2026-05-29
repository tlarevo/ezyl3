package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type FileLogReader struct {
	Runtime      Runtime
	PollInterval time.Duration
}

func (r FileLogReader) Read(service string) ([]byte, error) {
	path, err := r.path(service)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (r FileLogReader) Follow(ctx context.Context, service string, w io.Writer) error {
	path, err := r.path(service)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	interval := r.PollInterval
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if _, err := io.Copy(w, file); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := io.Copy(w, file); err != nil {
				return err
			}
		}
	}
}

func (r FileLogReader) path(service string) (string, error) {
	if service != "litellm" && service != "ngrok" {
		return "", fmt.Errorf("unknown log target %q", service)
	}
	return filepath.Join(r.Runtime.Path, "logs", service+".out.log"), nil
}
