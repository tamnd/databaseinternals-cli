// Package databaseinternals is the library behind the dbi command: the HTTP
// client, data models, and parsers for the "Database Internals" book site at
// https://www.databass.dev/.
//
// Chapter data is static (the book is published; chapters don't change).
// Errata are fetched live from the Squarespace JSON endpoint on the errata
// page.
package databaseinternals

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to databass.dev.
const DefaultUserAgent = "dbi/dev (+https://github.com/tamnd/databaseinternals-cli)"

// BaseURL is the companion site for the book.
const BaseURL = "https://www.databass.dev"

// Config holds constructor parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to the databass.dev Squarespace site.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// ─── Data models ─────────────────────────────────────────────────────────────

// Chapter is a single chapter from the book.
type Chapter struct {
	Number int    `json:"number"`
	Part   string `json:"part"`
	Title  string `json:"title"`
	Topics string `json:"topics"`
}

// Erratum is a single correction from the errata page.
type Erratum struct {
	Chapter int    `json:"chapter"`
	Section string `json:"section"`
	Page    int    `json:"page"`
	Text    string `json:"text"`
}

// BookInfo holds top-level book metadata.
type BookInfo struct {
	Title     string `json:"title"`
	Author    string `json:"author"`
	Publisher string `json:"publisher"`
	Year      int    `json:"year"`
	ISBN      string `json:"isbn"`
	URL       string `json:"url"`
	OReilly   string `json:"oreilly"`
}

// ─── Static data ─────────────────────────────────────────────────────────────

// Info is the static book metadata.
var Info = BookInfo{
	Title:     "Database Internals: A Deep Dive into How Distributed Data Systems Work",
	Author:    "Alex Petrov",
	Publisher: "O'Reilly Media",
	Year:      2019,
	ISBN:      "978-1-492-04034-7",
	URL:       "https://www.databass.dev",
	OReilly:   "https://www.oreilly.com/library/view/database-internals/9781492040330/",
}

// Chapters is the complete ordered list of chapters from the book.
var Chapters = []Chapter{
	{1, "I", "Introduction and Overview", "DBMS architecture, storage classifications, column vs row orientation, in-place update vs immutable storage"},
	{2, "I", "B-Tree Basics", "B-Tree structure, binary search, disk-based trees, fanout, height, node splits and merges"},
	{3, "I", "File Formats", "Binary data encoding, fixed vs variable-size data, page layout, slotted pages, cell layout"},
	{4, "I", "Implementing B-Trees", "Page cache, buffer management, WAL, concurrency, lock coupling, B-link trees"},
	{5, "I", "Transaction Processing and Recovery", "ACID, steal/force policies, ARIES, WAL recovery, concurrency control, isolation levels"},
	{6, "I", "B-Tree Variants", "Copy-on-write B-Trees, fractional cascading, FD-trees, Bw-Trees, cache-oblivious B-Trees"},
	{7, "I", "Log-Structured Storage", "LSM trees, SSTables, compaction strategies, Bitcask, WiscKey, LLAMA"},
	{8, "II", "Introduction and Overview", "Distributed systems basics, fallacies of distributed computing, links and processes"},
	{9, "II", "Failure Detection", "Heartbeats, Phi Accrual Failure Detector, SWIM protocol"},
	{10, "II", "Leader Election", "Bully algorithm, NextInLine, ring-based election, ZooKeeper"},
	{11, "II", "Replication and Consistency", "Consistency models, linearizability, serializability, session guarantees, causal consistency"},
	{12, "II", "Anti-Entropy and Dissemination", "Gossip protocols, epidemic broadcast, Merkle trees, read repair"},
	{13, "II", "Distributed Transactions", "Two-phase commit, three-phase commit, atomic commitment, Calvin, Spanner"},
	{14, "II", "Consensus", "FLP impossibility, Paxos, Multi-Paxos, Raft, Byzantine fault tolerance"},
}

// ─── Live errata fetch ────────────────────────────────────────────────────────

// squarespaceResp is the shape of the Squarespace ?format=json response.
type squarespaceResp struct {
	MainContent string `json:"mainContent"`
}

// Errata fetches the live errata list from databass.dev/errata.
func (c *Client) Errata(ctx context.Context) ([]Erratum, error) {
	u := c.cfg.BaseURL + "/errata?format=json"
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("fetch errata: %w", err)
	}
	var resp squarespaceResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode errata response: %w", err)
	}
	return ParseErrata(resp.MainContent), nil
}

// ─── HTML parsing ─────────────────────────────────────────────────────────────

var (
	reTag       = regexp.MustCompile(`<[^>]+>`)
	rePageNum   = regexp.MustCompile(`p\.\s*(\d+)`)
	reChapterN  = regexp.MustCompile(`(?i)^chapter\s+(\d+)\s*$`)
)

// ParseErrata parses the raw HTML mainContent from the Squarespace errata
// page and returns structured Erratum records. It handles chapter markers
// ("Chapter 1", "Chapter 2", ...) and preface/acknowledgement sections.
func ParseErrata(mainContent string) []Erratum {
	// Unescape HTML entities, then strip all tags.
	text := html.UnescapeString(mainContent)
	text = reTag.ReplaceAllString(text, "\n")

	// Split into lines; trim each.
	rawLines := strings.Split(text, "\n")
	var lines []string
	for _, l := range rawLines {
		l = strings.TrimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}

	var errata []Erratum
	currentChapter := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Detect chapter markers.
		if m := reChapterN.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			currentChapter = n
			continue
		}

		// Skip navigation/site lines.
		if isNavLine(line) {
			continue
		}

		// A correction line: must have "p." or "p.NN" to be parseable.
		if !strings.Contains(line, "p.") && !strings.Contains(line, "p ") {
			continue
		}

		// Parse "Section Title, p. NN. correction text"
		// or "Section Title, p.NN. correction text"
		section, page, corrText := parseCorrectionLine(line)
		if corrText == "" {
			continue
		}
		errata = append(errata, Erratum{
			Chapter: currentChapter,
			Section: section,
			Page:    page,
			Text:    corrText,
		})
	}
	return errata
}

// parseCorrectionLine splits a line like:
//
//	"DBMS Architecture, p. 10. Some correction text."
//
// into (section, page, text). Returns ("", 0, "") if it cannot parse.
func parseCorrectionLine(line string) (section string, page int, text string) {
	m := rePageNum.FindStringIndex(line)
	if m == nil {
		return "", 0, ""
	}
	before := strings.TrimRight(line[:m[0]], ", ")
	rest := line[m[0]:]

	// Extract page number.
	pm := rePageNum.FindStringSubmatch(rest)
	if pm == nil {
		return "", 0, ""
	}
	page, _ = strconv.Atoi(pm[1])

	// The text after the page reference.
	afterPage := rest[len(pm[0]):]
	afterPage = strings.TrimLeft(afterPage, "., ")
	if afterPage == "" {
		return "", 0, ""
	}

	return strings.TrimSpace(before), page, strings.TrimSpace(afterPage)
}

// isNavLine returns true for lines that are part of the page chrome or are
// bibliography reference entries, not correction content.
func isNavLine(line string) bool {
	navPhrases := []string{
		"Database Internals", "About Me", "Discord Channel",
		"Errata", "Submit new errata", "O'Reilly portal",
		"Despite thorough reviews", "This page contents errata",
		"Buy on", "Read on", "© 20",
	}
	for _, p := range navPhrases {
		if strings.Contains(line, p) {
			return true
		}
	}
	// Skip bibliography reference lines like "[STONE98] ..."
	if len(line) > 1 && line[0] == '[' {
		close := strings.Index(line, "]")
		if close > 1 && close < 20 {
			return true
		}
	}
	return false
}

// ─── HTTP client ─────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
