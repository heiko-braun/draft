package cli

import "github.com/spf13/cobra"

func newUpdateCmd() *cobra.Command {
	var check bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update draft to the latest release",
		RunE: func(cmd *cobra.Command, args []string) error {
			if check {
				return runCheck()
			}
			return runUpdate()
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "Check for a newer version without installing")

	return cmd
}
