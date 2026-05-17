package relay

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func Execute() error {
	return NewRootCommand().Execute()
}

func NewRootCommand() *cobra.Command {
	var listenAddr string
	cmd := &cobra.Command{
		Use:   "switchboard",
		Short: "Run the PhosphorNet relay/rendezvous service",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "serve",
		Short: "Start a minimal switchboard stub",
		RunE: func(cmd *cobra.Command, args []string) error {
			mux := http.NewServeMux()
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})
			fmt.Fprintf(cmd.OutOrStdout(), "switchboard listening on %s\n", listenAddr)
			return http.ListenAndServe(listenAddr, mux)
		},
	})
	cmd.PersistentFlags().StringVar(&listenAddr, "listen", ":7710", "listen address")
	return cmd
}
