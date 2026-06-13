package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/databaseinternals-cli/databaseinternals"
)

func (a *App) chaptersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chapters",
		Short: "List book chapters",
		Long: `List all 14 chapters from "Database Internals" with their part number,
title, and a summary of key topics covered.

Part I covers Storage Engines (chapters 1-7).
Part II covers Distributed Systems (chapters 8-14).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			chapters := databaseinternals.Chapters
			n := a.effectiveLimit(len(chapters))
			if n < len(chapters) {
				chapters = chapters[:n]
			}
			return a.renderOrEmpty(chapters, len(chapters))
		},
	}
}
