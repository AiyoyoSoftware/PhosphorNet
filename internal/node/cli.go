package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

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
		systemPaths   bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter node configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			passport, err := identity.Generate(name)
			if err != nil {
				return err
			}
			cfg := defaultInitNodeConfig(out, cmd.Flags().Changed("out"), systemPaths)
			cfg.Name = name
			cfg.NodeID = passport.PublicKey
			cfg.PrivateKey = passport.PrivateKey
			admin, created, err := ensureAdminPassport(adminPassport)
			if err != nil {
				return err
			}
			cfg.Access.Admins = []string{admin.PublicKey}
			if err := app.EnsureParentDir(out); err != nil {
				return err
			}
			if err := app.EnsureParentDir(cfg.Database); err != nil {
				return err
			}
			if err := config.SaveNodeConfig(out, cfg); err != nil {
				return err
			}
			if err := seedDefaultStationPolicy(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote node config to %s\n", out)
			fmt.Fprintf(cmd.OutOrStdout(), "default station policy disables strategy_demo and action_demo until an admin enables them\n")
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
	cmd.Flags().StringVar(&out, "out", app.DefaultNodeConfigPath(), "config path to write")
	cmd.Flags().StringVar(&adminPassport, "admin-passport", app.DefaultPassportPath(), "admin passport path to create or reuse")
	cmd.Flags().BoolVar(&systemPaths, "system-paths", false, "use system doors and database paths in the generated node config")
	return cmd
}

func defaultInitNodeConfig(out string, outChanged bool, systemPaths bool) config.NodeConfig {
	if systemPaths || (!outChanged && out == app.SystemNodeConfigPath) {
		return config.DefaultSystemNodeConfig()
	}
	if outChanged {
		cfg := config.DefaultLocalNodeConfig()
		if dir := filepath.Dir(out); dir != "." && dir != "" {
			cfg.Database = filepath.Join(dir, app.DefaultDatabaseName)
		}
		return cfg
	}
	return config.DefaultNodeConfig()
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
	changed := false
	for _, doorID := range []string{"strategy_demo", "action_demo"} {
		if _, exists := disabled[doorID]; exists {
			continue
		}
		disabled[doorID] = true
		changed = true
	}
	if !changed {
		return nil
	}

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
	var (
		configPath         string
		auditLogFile       string
		auditLogMaxBytes   int64
		auditLogMaxBackups int
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the node daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadNodeConfig(configPath)
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("config") {
				cfg = config.ApplyHomeOverrides(cfg)
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
			if cfg.Actiond.Enabled {
				log.Printf("phosphord action delegation enabled at unix://%s", cfg.Actiond.Socket)
			} else {
				log.Printf("phosphord action delegation disabled")
			}

			var auditFile *rotatingAuditFile
			if auditLogFile != "" {
				auditFile, err = openRotatingAuditFile(auditLogFile, auditLogMaxBytes, auditLogMaxBackups)
				if err != nil {
					return fmt.Errorf("open audit log file: %w", err)
				}
				defer auditFile.Close()
				log.Printf("phosphord audit log file=%s max_bytes=%d max_backups=%d", auditLogFile, auditLogMaxBytes, auditLogMaxBackups)
			}

			nodeServer := newServerWithOptions(cfg, doorManifests, store, serverOptions{
				AuditLogFile:     auditFile,
				AuditLogMaxBytes: auditLogMaxBytes,
			})
			if err := nodeServer.rememberNodeIdentity(context.Background()); err != nil {
				return fmt.Errorf("remember node identity: %w", err)
			}

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

	cmd.Flags().StringVar(&configPath, "config", app.DefaultNodeConfigPath(), "node config path")
	cmd.Flags().StringVar(&auditLogFile, "audit-log-file", "", "optional JSONL file to append audit events to")
	cmd.Flags().Int64Var(&auditLogMaxBytes, "audit-log-max-bytes", 0, "maximum audit log bytes for SQLite retention and optional file rotation; 0 disables size limiting")
	cmd.Flags().IntVar(&auditLogMaxBackups, "audit-log-file-max-backups", 5, "number of rotated audit log files to keep when audit log rotation is enabled")
	return cmd
}
