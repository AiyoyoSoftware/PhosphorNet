package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"phosphornet/internal/app"
	"phosphornet/internal/config"
	"phosphornet/internal/identity"
	"phosphornet/internal/protocol"
	"phosphornet/internal/runtime"
	"phosphornet/internal/storage"
)

func Execute() error {
	return NewRootCommand().Execute()
}

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "phosphord",
		Short: "Run a PhosphorNet node daemon",
	}
	root.AddCommand(newInitCommand(), newServeCommand())
	return root
}

func newInitCommand() *cobra.Command {
	var (
		name          string
		out           string
		adminPassport string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter node configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			passport, err := identity.Generate(name)
			if err != nil {
				return err
			}
			cfg := config.DefaultNodeConfig()
			cfg.Name = name
			cfg.NodeID = passport.PublicKey
			cfg.PrivateKey = passport.PrivateKey
			admin, created, err := ensureAdminPassport(adminPassport)
			if err != nil {
				return err
			}
			cfg.Access.Admins = []string{admin.PublicKey}
			if err := config.SaveNodeConfig(out, cfg); err != nil {
				return err
			}
			if err := seedDefaultStationPolicy(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote node config to %s\n", out)
			fmt.Fprintf(cmd.OutOrStdout(), "default station policy disables strategy_demo until an admin enables it\n")
			if created {
				fmt.Fprintf(cmd.OutOrStdout(), "created admin passport at %s\n", adminPassport)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "using existing admin passport at %s\n", adminPassport)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "admin fingerprint: %s\n", admin.Fingerprint())
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "localbox", "station display name")
	cmd.Flags().StringVar(&out, "out", "node.toml", "config path to write")
	cmd.Flags().StringVar(&adminPassport, "admin-passport", app.DefaultPassportPath(), "admin passport path to create or reuse")
	return cmd
}

func ensureAdminPassport(path string) (*identity.Passport, bool, error) {
	passport, err := identity.Load(path)
	if err == nil {
		return passport, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	if err := app.EnsureParentDir(path); err != nil {
		return nil, false, err
	}
	passport, err = identity.Generate("admin")
	if err != nil {
		return nil, false, err
	}
	if err := identity.Save(path, passport); err != nil {
		return nil, false, err
	}
	return passport, true, nil
}

func seedDefaultStationPolicy(cfg config.NodeConfig) error {
	store, err := storage.OpenSQLite(cfg.Database)
	if err != nil {
		return fmt.Errorf("seed default station policy: %w", err)
	}
	defer store.Close()

	return seedDefaultStationPolicyStore(context.Background(), store)
}

func seedDefaultStationPolicyStore(ctx context.Context, store *storage.Store) error {
	if store == nil {
		return fmt.Errorf("seed default station policy: storage is required")
	}
	state, err := store.LoadScopedState(ctx, adminDoorID, storage.StateScopeIDs{Global: "global"})
	if err != nil {
		return fmt.Errorf("seed default station policy: %w", err)
	}

	disabled := boolMapFromAnyMap(state.Global["disabled_doors"])
	if disabled["strategy_demo"] {
		return nil
	}
	if _, exists := disabled["strategy_demo"]; exists {
		return nil
	}
	disabled["strategy_demo"] = true

	return store.ApplyStateOps(ctx, adminDoorID, storage.StateScopeIDs{Global: "global"}, "admin", []protocol.StateOp{
		{
			Scope: protocol.StateScopeGlobal,
			Op:    protocol.StateOpSet,
			Key:   "disabled_doors",
			Value: disabled,
		},
	})
}

func newServeCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the node daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadNodeConfig(configPath)
			if err != nil {
				return err
			}

			doorManifests, err := runtime.LoadDoorManifests(cfg.DoorsDir)
			if err != nil {
				return err
			}

			store, err := storage.OpenSQLite(cfg.Database)
			if err != nil {
				return err
			}
			defer store.Close()
			if store.Created() {
				if err := seedDefaultStationPolicyStore(context.Background(), store); err != nil {
					return err
				}
			}
			schemaVersion, err := store.SchemaVersion(context.Background())
			if err != nil {
				return err
			}
			log.Printf("phosphord database path=%s schema_version=%d", store.Path(), schemaVersion)

			nodeServer := newServer(cfg, doorManifests, store)

			mux := http.NewServeMux()
			mux.HandleFunc("/ws", nodeServer.handleWS)
			mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			httpServer := &http.Server{
				Addr:    cfg.ListenAddr,
				Handler: mux,
			}
			tlsCfg, err := tlsConfigForNode(cfg)
			if err != nil {
				return err
			}
			if tlsCfg != nil {
				httpServer.TLSConfig = tlsCfg
				log.Printf("phosphord listening on %s with tls", cfg.ListenAddr)
				return httpServer.ListenAndServeTLS("", "")
			}

			log.Printf("phosphord listening on %s without tls", cfg.ListenAddr)
			return httpServer.ListenAndServe()
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "node.toml", "node config path")
	return cmd
}
