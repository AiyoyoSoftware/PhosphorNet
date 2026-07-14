package action

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"phosphornet/internal/app"
)

func Execute() error {
	return NewRootCommand().Execute()
}

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "phosphor-actiond",
		Short:         "Run explicitly allowlisted host actions for PhosphorNet doors",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.AddCommand(newInitCommand(), newServeCommand())
	return root
}

func newInitCommand() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter action rules configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := DefaultConfig()
			if cmd.Flags().Changed("out") && out != app.SystemActiondConfigPath {
				absoluteOut, err := filepath.Abs(out)
				if err != nil {
					return err
				}
				cfg.Socket = filepath.Join(filepath.Dir(absoluteOut), "actiond.sock")
			}
			if err := app.EnsureParentDir(out); err != nil {
				return err
			}
			if err := SaveConfig(out, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote actiond config to %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", app.DefaultActiondConfigPath(), "actiond config path to write")
	return cmd
}

func newServeCommand() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the action JSON protocol over a Unix socket",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(configPath)
			if err != nil {
				return err
			}
			return NewServer(cfg).Serve(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&configPath, "config", app.DefaultActiondConfigPath(), "actiond config path")
	return cmd
}
