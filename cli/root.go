// Package cli builds the dbi command tree on top of the databaseinternals library.
package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/tamnd/databaseinternals-cli/databaseinternals"
)

// Build metadata, injected via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// App holds shared state threaded through every command.
type App struct {
	cfg    databaseinternals.Config
	client *databaseinternals.Client

	format   string
	fields   []string
	noHeader bool
	template string
	limit    int
	quiet    bool
}

// Root builds the root command and its subtree.
func Root() *cobra.Command {
	app := &App{cfg: databaseinternals.DefaultConfig()}

	root := &cobra.Command{
		Use:   "dbi",
		Short: "Browse Database Internals book content",
		Long: `dbi reads content from the companion site for "Database Internals: A Deep Dive
into How Distributed Data Systems Work" by Alex Petrov (O'Reilly, 2019).

It returns structured records from the book's chapter list, live errata, and
book metadata as table, JSON, JSONL, CSV, TSV, or raw text.

dbi is an independent tool and is not affiliated with Alex Petrov, O'Reilly
Media, or Squarespace.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return app.setup()
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&app.format, "output", "o", "auto", "output: table|json|jsonl|csv|tsv|url|raw (auto=table on TTY, jsonl piped)")
	pf.StringSliceVar(&app.fields, "fields", nil, "comma-separated columns to include")
	pf.BoolVar(&app.noHeader, "no-header", false, "omit the header row in table/csv/tsv")
	pf.StringVar(&app.template, "template", "", "Go text/template applied per record")
	pf.IntVarP(&app.limit, "limit", "n", 0, "limit number of records (0 = command default)")
	pf.BoolVarP(&app.quiet, "quiet", "q", false, "suppress progress on stderr")

	pf.DurationVar(&app.cfg.Rate, "delay", app.cfg.Rate, "minimum spacing between requests")
	pf.DurationVar(&app.cfg.Timeout, "timeout", app.cfg.Timeout, "per-request timeout")
	pf.IntVar(&app.cfg.Retries, "retries", app.cfg.Retries, "retry attempts on 429/5xx")
	pf.StringVar(&app.cfg.UserAgent, "user-agent", app.cfg.UserAgent, "User-Agent sent with each request")

	root.AddCommand(
		app.chaptersCmd(),
		app.errataCmd(),
		app.infoCmd(),
		newVersionCmd(),
	)
	return root
}

func (a *App) setup() error {
	if a.format == "" || a.format == "auto" {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			a.format = string(FormatTable)
		} else {
			a.format = string(FormatJSONL)
		}
	}
	if !Format(a.format).Valid() {
		return codeError(exitUsage, fmt.Errorf("unknown output format %q", a.format))
	}
	a.client = databaseinternals.NewClient(a.cfg)
	return nil
}

func (a *App) render(records any) error {
	r := NewRenderer(os.Stdout, Format(a.format), a.fields, a.noHeader, a.template)
	return r.Render(records)
}

func (a *App) renderOrEmpty(records any, n int) error {
	if err := a.render(records); err != nil {
		return err
	}
	if n == 0 {
		return codeError(exitNoData, nil)
	}
	return nil
}

func (a *App) progressf(format string, args ...any) {
	if a.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (a *App) effectiveLimit(def int) int {
	if a.limit > 0 {
		return a.limit
	}
	return def
}
