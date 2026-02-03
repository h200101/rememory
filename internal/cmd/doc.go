package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docCmd = &cobra.Command{
	Use:    "doc <output-dir>",
	Short:  "Generate man pages and markdown documentation",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE:   runDoc,
}

var docFormat string

func init() {
	docCmd.Flags().StringVar(&docFormat, "format", "man", "Output format: man, markdown")
	rootCmd.AddCommand(docCmd)
}

func runDoc(cmd *cobra.Command, args []string) error {
	outputDir := args[0]

	switch docFormat {
	case "man":
		header := &doc.GenManHeader{
			Title:   "REMEMORY",
			Section: "1",
		}
		if err := doc.GenManTree(rootCmd, header, outputDir); err != nil {
			return fmt.Errorf("generating man pages: %w", err)
		}
		fmt.Printf("Man pages generated in %s\n", outputDir)

	case "markdown":
		if err := doc.GenMarkdownTree(rootCmd, outputDir); err != nil {
			return fmt.Errorf("generating markdown: %w", err)
		}
		fmt.Printf("Markdown docs generated in %s\n", outputDir)

	default:
		return fmt.Errorf("unknown format: %s (use 'man' or 'markdown')", docFormat)
	}

	return nil
}
