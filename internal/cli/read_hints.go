package cli

import (
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/spf13/cobra"
)

// Bind the named download's record hint to its validated positional ID. Keeping
// this in command construction leaves the shared download implementation alone.
func (s *session) readDownloadCommand(row commands.Row) *cobra.Command {
	cmd := s.downloadCommand(&row)
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		row.Next = []string{"slowbooks " + strings.Fields(row.Name)[0] + " show " + args[0]}
		return run(cmd, args)
	}
	return cmd
}
