package cli

import (
	"embed"
	"os"

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
		Use:     "draft",
		Short:   "Draft your specs before you code",
		Long:    `draft helps you set up specification-driven development in any repository by copying the necessary command files and templates for AI coding assistants like Claude Code.`,
		Version: appVersion,
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// Skip notice for the update-related commands themselves.
			name := cmd.Name()
			if name == "update" || name == "version" {
				return
			}
			maybePrintUpdateNotice(os.Stderr, appVersion)
		},
	}

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newIndexCmd())
	rootCmd.AddCommand(newSearchCmd())

	return rootCmd.Execute()
}
