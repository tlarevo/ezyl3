package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUsageStoreSummarizesEventsByDateModelAndProvider(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
	store, err := OpenUsageStore(dbPath)
	if err != nil {
		t.Fatalf("OpenUsageStore returned error: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	events := []UsageEvent{
		{CreatedAt: now.Add(-2 * time.Hour), Model: "litellm-simple", Provider: "huggingface", PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30, CostUSD: 0.003, DurationMS: 1200, Status: "success"},
		{CreatedAt: now.Add(-1 * time.Hour), Model: "litellm-simple", Provider: "huggingface", PromptTokens: 5, CompletionTokens: 15, TotalTokens: 20, CostUSD: 0.002, DurationMS: 900, Status: "success"},
		{CreatedAt: now.Add(-25 * time.Hour), Model: "litellm-medium", Provider: "ollama", PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300, CostUSD: 0, DurationMS: 3000, Status: "success"},
	}
	for _, event := range events {
		if err := store.Record(event); err != nil {
			t.Fatalf("Record returned error: %v", err)
		}
	}

	summary, err := store.Summary(now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}

	if summary.Requests != 2 {
		t.Fatalf("Requests = %d, want 2", summary.Requests)
	}
	if summary.PromptTokens != 15 || summary.CompletionTokens != 35 || summary.TotalTokens != 50 {
		t.Fatalf("token totals = %d/%d/%d", summary.PromptTokens, summary.CompletionTokens, summary.TotalTokens)
	}
	if summary.CostUSD != 0.005 {
		t.Fatalf("CostUSD = %v, want 0.005", summary.CostUSD)
	}
	if len(summary.TopModels) != 1 || summary.TopModels[0].Name != "litellm-simple" || summary.TopModels[0].Requests != 2 || summary.TopModels[0].TotalTokens != 50 {
		t.Fatalf("TopModels = %#v", summary.TopModels)
	}
	if len(summary.TopProviders) != 1 || summary.TopProviders[0].Name != "huggingface" || summary.TopProviders[0].Requests != 2 {
		t.Fatalf("TopProviders = %#v", summary.TopProviders)
	}
}

func TestUsageSummaryHandlesEmptyDatabase(t *testing.T) {
	store, err := OpenUsageStore(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("OpenUsageStore returned error: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	summary, err := store.Summary(now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}
	if summary.Requests != 0 || summary.TotalTokens != 0 || summary.CostUSD != 0 {
		t.Fatalf("empty summary = %#v", summary)
	}
}
