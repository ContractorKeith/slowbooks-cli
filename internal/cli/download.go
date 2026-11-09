package cli

import (
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/ContractorKeith/slowbooks-cli/internal/client"
	"github.com/ContractorKeith/slowbooks-cli/internal/commands"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
	"github.com/spf13/cobra"
)

type downloadData struct {
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
}

func (s *session) downloadCommand(row *commands.Row) *cobra.Command {
	var destination string
	var params []string
	var force bool
	use := "download <path>"
	if row != nil {
		parts := strings.Fields(row.Name)
		use = parts[len(parts)-1] + " <id>"
	}
	cmd := &cobra.Command{Use: use, Short: "Download a server file", Args: usageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		var raw io.Writer
		if destination == "-" {
			raw = s.w.DownloadStream()
		}
		target := args[0]
		if row != nil {
			if err := client.ValidatePathParam(target); err != nil {
				return err
			}
			target = strings.ReplaceAll(row.Path, "{"+row.Args[0]+"}", url.PathEscape(target))
		}
		u, err := url.Parse(target)
		if err != nil {
			return &output.Error{Code: "usage", Message: "invalid download path"}
		}
		query, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return &output.Error{Code: "usage", Message: "malformed query string"}
		}
		for _, param := range params {
			k, v, ok := strings.Cut(param, "=")
			if !ok || k == "" {
				return &output.Error{Code: "usage", Message: "--param requires k=v"}
			}
			if strings.Contains(u.Path, "{"+k+"}") {
				if err := client.ValidatePathParam(v); err != nil {
					return err
				}
				u.Path = strings.ReplaceAll(u.Path, "{"+k+"}", v)
				u.RawPath = ""
			} else {
				query.Add(k, v)
			}
		}
		if strings.ContainsAny(u.Path, "{}") {
			return &output.Error{Code: "usage", Message: "missing path parameter"}
		}
		u.RawQuery = query.Encode()
		_, cl, err := s.connect(cmd)
		if err != nil {
			return err
		}
		var staged *output.DownloadFile
		if raw == nil {
			staged, err = output.NewDownloadFile(destination)
			if err != nil {
				return err
			}
			defer staged.Abort()
			raw = staged
		}
		response, err := cl.Download(cmd.Context(), u.String(), raw)
		if err != nil {
			return err
		}
		if cmd.Context().Err() != nil {
			return &output.Error{Code: "network", Message: "download cancelled"}
		}
		filename := safeFilename(response.Filename)
		if filename == "" {
			filename = safeFilename(path.Base(u.Path))
		}
		if filename == "" {
			filename = "download"
		}
		saved := destination
		if saved == "" {
			saved = filename
		}
		if staged != nil {
			saved, err = staged.Save(cmd.Context(), filename, force)
			if err != nil {
				return err
			}
		}
		if destination == "-" && !s.w.JSON {
			return nil
		}
		if destination == "-" {
			s.w.Quiet = false
		}
		var next []string
		if row != nil {
			next = row.Next
		}
		return s.w.Success(downloadData{saved, response.Bytes, response.ContentType, filename}, "Downloaded "+filename, next)
	}}
	cmd.Flags().StringVarP(&destination, "output", "o", "", "Destination file, or - for stdout")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing file")
	if row == nil {
		cmd.Flags().StringArrayVar(&params, "param", nil, "Path or query parameter k=v (repeatable)")
	}
	return cmd
}
