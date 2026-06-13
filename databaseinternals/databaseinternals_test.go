package databaseinternals_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/databaseinternals-cli/databaseinternals"
)

func TestChapters(t *testing.T) {
	if len(databaseinternals.Chapters) != 14 {
		t.Fatalf("expected 14 chapters, got %d", len(databaseinternals.Chapters))
	}
	part1, part2 := 0, 0
	for _, ch := range databaseinternals.Chapters {
		switch ch.Part {
		case "I":
			part1++
		case "II":
			part2++
		default:
			t.Errorf("chapter %d has unknown part %q", ch.Number, ch.Part)
		}
		if ch.Title == "" {
			t.Errorf("chapter %d has empty title", ch.Number)
		}
		if ch.Topics == "" {
			t.Errorf("chapter %d has empty topics", ch.Number)
		}
	}
	if part1 != 7 {
		t.Errorf("Part I: got %d chapters, want 7", part1)
	}
	if part2 != 7 {
		t.Errorf("Part II: got %d chapters, want 7", part2)
	}
}

func TestChapterNumbers(t *testing.T) {
	for i, ch := range databaseinternals.Chapters {
		want := i + 1
		if ch.Number != want {
			t.Errorf("chapters[%d].Number = %d, want %d", i, ch.Number, want)
		}
	}
}

func TestInfo(t *testing.T) {
	if databaseinternals.Info.Author != "Alex Petrov" {
		t.Errorf("Info.Author = %q, want %q", databaseinternals.Info.Author, "Alex Petrov")
	}
	if databaseinternals.Info.Year != 2019 {
		t.Errorf("Info.Year = %d, want 2019", databaseinternals.Info.Year)
	}
	if databaseinternals.Info.URL == "" {
		t.Error("Info.URL is empty")
	}
}

func TestParseErrata(t *testing.T) {
	// Simulate a fragment of the errata mainContent HTML.
	sample := `<p>Chapter 1</p>
<p>DBMS Architecture, p. 10. "The execution plan is handled..." should be "Plan is carried out...".</p>
<p>Index Files, p. 19. Text before Figure 1-5 should include two labeled items.</p>
<p>Chapter 2</p>
<p>B-Tree Lookup Complexity, p. 37. All variables named "K" should be "N".</p>`

	errata := databaseinternals.ParseErrata(sample)
	if len(errata) == 0 {
		t.Fatal("ParseErrata returned no errata from sample HTML")
	}

	var ch1, ch2 []databaseinternals.Erratum
	for _, e := range errata {
		switch e.Chapter {
		case 1:
			ch1 = append(ch1, e)
		case 2:
			ch2 = append(ch2, e)
		}
	}

	if len(ch1) < 1 {
		t.Errorf("expected at least 1 chapter 1 erratum, got %d", len(ch1))
	}
	if len(ch2) < 1 {
		t.Errorf("expected at least 1 chapter 2 erratum, got %d", len(ch2))
	}

	// Spot check the first chapter 1 erratum.
	e := ch1[0]
	if e.Section == "" {
		t.Error("erratum section is empty")
	}
	if e.Page == 0 {
		t.Error("erratum page is 0, expected non-zero")
	}
	if e.Text == "" {
		t.Error("erratum text is empty")
	}
}

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"mainContent": "<p>ok</p>"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := databaseinternals.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	client := databaseinternals.NewClient(cfg)
	errata, err := client.Errata(context.Background())
	if err != nil {
		t.Fatalf("Errata: %v", err)
	}
	// The sample mainContent has no parseable corrections, that's fine.
	_ = errata
}

func TestClientRetries(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"mainContent": ""}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := databaseinternals.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5

	start := time.Now()
	client := databaseinternals.NewClient(cfg)
	_, err := client.Errata(context.Background())
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}
