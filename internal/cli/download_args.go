package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// rawDownloadOptions reserves stdout before cobra can reject arguments or flags.
func rawDownloadOptions(root *cobra.Command, args []string) (bool, bool) {
	cmd, _, _ := root.Find(args)
	if cmd == nil || (cmd.Name() != "download" && !(cmd.Name() == "pdf" && cmd.Parent().Name() == "invoices")) {
		return false, false
	}
	destination := ""
	jsonOutput := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		switch {
		case arg == "--json" || arg == "--json=true":
			jsonOutput = true
		case arg == "--json=false":
			jsonOutput = false
		case arg == "-o" || arg == "--output":
			if i+1 < len(args) {
				i++
				destination = args[i]
			}
		case strings.HasPrefix(arg, "--output="):
			destination = strings.TrimPrefix(arg, "--output=")
		case strings.HasPrefix(arg, "-o"):
			destination = strings.TrimPrefix(strings.TrimPrefix(arg, "-o"), "=")
		default:
			// Values of other known flags are not output flags.
			name := strings.TrimPrefix(arg, "--")
			flag := cmd.Flags().Lookup(name)
			if flag == nil {
				flag = root.PersistentFlags().Lookup(name)
			}
			if flag != nil && flag.NoOptDefVal == "" && !strings.Contains(arg, "=") && i+1 < len(args) {
				i++
			}
		}
	}
	return destination == "-", jsonOutput
}
