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
		Use:     "draft",
		Short:   "Draft your specs before you code",
		Long:    `draft helps you set up specification-driven development in any repository by copying the necessary command files and templates for AI coding assistants like Claude Code.`,
		Version: appVersion,
	}

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd.Execute()
}
