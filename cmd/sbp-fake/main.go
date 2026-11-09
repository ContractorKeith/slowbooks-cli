package main

import (
	"errors"
	"flag"
	"net"
	"net/http"
	"os"

	"github.com/ContractorKeith/slowbooks-cli/internal/fakeserver"
	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

func main() { os.Exit(run()) }

func run() int {
	w := output.New(os.Stdout, os.Stderr)
	flags := flag.NewFlagSet("sbp-fake", flag.ContinueOnError)
	flags.SetOutput(w.HelpWriter())
	addr := flags.String("listen", "127.0.0.1:0", "Listen address")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return w.Failure(&output.Error{Code: "usage", Message: err.Error()})
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return w.Failure(err)
	}
	if err := w.Text("http://" + listener.Addr().String() + "\n"); err != nil {
		return w.Failure(err)
	}
	err = http.Serve(listener, fakeserver.New())
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return w.Failure(err)
	}
	return output.ExitCode(nil)
}
