package cli

import (
	"embed"

	"github.com/spf13/cobra"
)

var (
	templateFS embed.FS
	appVersion string
)

func Execute(templates embed.FS, version string) error {
	templateFS = templates
	appVersion = version

	rootCmd := &cobra.Command{
		Use:     "claudespec",
		Short:   "Bootstrap Claude spec-driven development workflow",
		Long:    `claudespec helps you set up spec-driven development in any repository by copying the necessary Claude command files and templates.`,
		Version: appVersion,
	}

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd.Execute()
}
