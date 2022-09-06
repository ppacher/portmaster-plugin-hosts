package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/ppacher/portmaster-plugin-hosts/resolver"
	"github.com/safing/portmaster/plugin/framework"
	"github.com/safing/portmaster/plugin/framework/cmds"
	"github.com/safing/portmaster/plugin/shared"
	"github.com/safing/portmaster/plugin/shared/proto"
	"github.com/spf13/cobra"
)

type Config struct {
	HostsPath string `json:"hostsFile"`
}

func main() {
	cmd := &cobra.Command{
		Use:   "portmaster-plugin-hosts",
		Short: "Add support to resolve domains using /etc/hosts and similar",
		Run: func(_ *cobra.Command, args []string) {
			var hostsResolver *resolver.HostsResolver

			framework.RegisterResolver(
				// we encapsulate the resolver in an additional function so we can
				// create the resolver when the framework is actually initialized.
				framework.ResolverFunc(func(ctx context.Context, d *proto.DNSQuestion, c *proto.Connection) (*proto.DNSResponse, error) {
					return hostsResolver.Resolve(ctx, d, c)
				}),
			)

			framework.OnInit(func(ctx context.Context) error {
				var cfg Config

				if framework.HasStaticConfig() {
					if err := framework.ParseStaticConfig(&cfg); err != nil {
						return err
					}
				}

				var err error
				hostsResolver, err = resolver.NewHostsResolver(cfg.HostsPath)
				if err != nil {
					return err
				}

				return nil
			})

			framework.Serve()
		},
	}

	cfg := &cmds.InstallCommandConfig{
		PluginName: "portmaster-plugin-hosts",
		Types: []shared.PluginType{
			shared.PluginTypeResolver,
		},
	}

	var hostsPath string
	installCmd := cmds.InstallCommand(cfg)

	installCmd.Flags().StringVar(&hostsPath, "path", "", "Path to the hosts file")
	installCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if hostsPath != "" {
			blob, err := json.MarshalIndent(Config{HostsPath: hostsPath}, "", "    ")
			if err != nil {
				hclog.L().Error("failed to create static configuration", "error", err)
				os.Exit(1)
			}

			cfg.StaticConfig = blob
		}
	}

	cmd.AddCommand(installCmd)

	if err := cmd.Execute(); err != nil {
		hclog.L().Error(err.Error())
		os.Exit(1)
	}
}
