package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/databaseinternals-cli/databaseinternals"
)

func (a *App) infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show book metadata",
		Long:  `Print the title, author, publisher, year, ISBN, and buy links for the book.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.render(databaseinternals.Info)
		},
	}
}
