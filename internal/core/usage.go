package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const UsageDBFileName = "usage.sqlite"

type UsageEvent struct {
	CreatedAt        time.Time `json:"created_at"`
	Model            string    `json:"model"`
	Provider         string    `json:"provider"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	DurationMS       int64     `json:"duration_ms"`
	Status           string    `json:"status"`
	Error            string    `json:"error,omitempty"`
}

type UsageSummary struct {
	Since            time.Time        `json:"since"`
	Until            time.Time        `json:"until"`
	Requests         int64            `json:"requests"`
	PromptTokens     int64            `json:"prompt_tokens"`
	CompletionTokens int64            `json:"completion_tokens"`
	TotalTokens      int64            `json:"total_tokens"`
	CostUSD          float64          `json:"cost_usd"`
	TopModels        []UsageBreakdown `json:"top_models"`
	TopProviders     []UsageBreakdown `json:"top_providers"`
}

type UsageBreakdown struct {
	Name        string  `json:"name"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

type UsageStore struct {
	db *sql.DB
}

func UsageDBPath(runtime Runtime) string {
	return filepath.Join(runtime.Path, UsageDBFileName)
}

func OpenUsageStore(path string) (*UsageStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &UsageStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func SummarizeUsage(runtime Runtime, since, until time.Time) (UsageSummary, error) {
	path := UsageDBPath(runtime)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return UsageSummary{Since: since, Until: until}, nil
	} else if err != nil {
		return UsageSummary{}, err
	}
	store, err := OpenUsageStore(path)
	if err != nil {
		return UsageSummary{}, err
	}
	defer store.Close()
	return store.Summary(since, until)
}

func (s *UsageStore) Close() error {
	return s.db.Close()
}

func (s *UsageStore) Record(event UsageEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(`INSERT INTO usage_events (
		created_at, model, provider, prompt_tokens, completion_tokens, total_tokens, cost_usd, duration_ms, status, error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.CreatedAt.UTC().Format(time.RFC3339Nano),
		event.Model,
		event.Provider,
		event.PromptTokens,
		event.CompletionTokens,
		event.TotalTokens,
		event.CostUSD,
		event.DurationMS,
		event.Status,
		event.Error,
	)
	return err
}

func (s *UsageStore) Summary(since, until time.Time) (UsageSummary, error) {
	summary := UsageSummary{Since: since, Until: until}
	row := s.db.QueryRow(`SELECT
		COUNT(*),
		COALESCE(SUM(prompt_tokens), 0),
		COALESCE(SUM(completion_tokens), 0),
		COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(cost_usd), 0)
		FROM usage_events
		WHERE created_at >= ? AND created_at < ?`,
		since.UTC().Format(time.RFC3339Nano),
		until.UTC().Format(time.RFC3339Nano),
	)
	if err := row.Scan(&summary.Requests, &summary.PromptTokens, &summary.CompletionTokens, &summary.TotalTokens, &summary.CostUSD); err != nil {
		return UsageSummary{}, err
	}
	var err error
	summary.TopModels, err = s.breakdown("model", since, until)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.TopProviders, err = s.breakdown("provider", since, until)
	if err != nil {
		return UsageSummary{}, err
	}
	return summary, nil
}

func (s *UsageStore) init() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS usage_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TEXT NOT NULL,
		model TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL DEFAULT '',
		prompt_tokens INTEGER NOT NULL DEFAULT 0,
		completion_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT '',
		error TEXT NOT NULL DEFAULT ''
	)`)
	return err
}

func (s *UsageStore) breakdown(column string, since, until time.Time) ([]UsageBreakdown, error) {
	if column != "model" && column != "provider" {
		return nil, fmt.Errorf("unsupported usage breakdown column %q", column)
	}
	rows, err := s.db.Query(fmt.Sprintf(`SELECT
		%s,
		COUNT(*),
		COALESCE(SUM(total_tokens), 0),
		COALESCE(SUM(cost_usd), 0)
		FROM usage_events
		WHERE created_at >= ? AND created_at < ?
		GROUP BY %s
		ORDER BY SUM(total_tokens) DESC, COUNT(*) DESC
		LIMIT 5`, column, column),
		since.UTC().Format(time.RFC3339Nano),
		until.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UsageBreakdown
	for rows.Next() {
		var item UsageBreakdown
		if err := rows.Scan(&item.Name, &item.Requests, &item.TotalTokens, &item.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
