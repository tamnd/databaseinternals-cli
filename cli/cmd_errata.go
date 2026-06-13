package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tamnd/databaseinternals-cli/databaseinternals"
)

func (a *App) errataCmd() *cobra.Command {
	var chapter int

	cmd := &cobra.Command{
		Use:   "errata",
		Short: "Fetch live book errata from databass.dev",
		Long: `Fetch the current errata from https://www.databass.dev/errata and return
structured records with chapter number, section, page, and correction text.

The errata page is maintained by the author and updated as corrections are found.

Use --chapter to filter to a specific chapter (0 = preface/acknowledgements).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.progressf("fetching errata from databass.dev...")
			errata, err := a.client.Errata(cmd.Context())
			if err != nil {
				return codeError(exitError, fmt.Errorf("errata: %w", err))
			}
			if chapter >= 0 {
				var filtered []databaseinternals.Erratum
				for _, e := range errata {
					if e.Chapter == chapter {
						filtered = append(filtered, e)
					}
				}
				errata = filtered
			}
			n := a.effectiveLimit(len(errata))
			if n < len(errata) {
				errata = errata[:n]
			}
			return a.renderOrEmpty(errata, len(errata))
		},
	}

	cmd.Flags().IntVarP(&chapter, "chapter", "c", -1, "filter by chapter number (0=preface, -1=all)")
	return cmd
}
