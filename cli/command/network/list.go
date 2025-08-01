package network

import (
	"context"
	"sort"

	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/completion"
	"github.com/docker/cli/cli/command/formatter"
	flagsHelper "github.com/docker/cli/cli/flags"
	"github.com/docker/cli/opts"
	"github.com/fvbommel/sortorder"
	"github.com/moby/moby/api/types/network"
	"github.com/spf13/cobra"
)

type listOptions struct {
	quiet   bool
	noTrunc bool
	format  string
	filter  opts.FilterOpt
}

func newListCommand(dockerCli command.Cli) *cobra.Command {
	options := listOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS]",
		Aliases: []string{"list"},
		Short:   "List networks",
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), dockerCli, options)
		},
		ValidArgsFunction: completion.NoComplete,
	}

	flags := cmd.Flags()
	flags.BoolVarP(&options.quiet, "quiet", "q", false, "Only display network IDs")
	flags.BoolVar(&options.noTrunc, "no-trunc", false, "Do not truncate the output")
	flags.StringVar(&options.format, "format", "", flagsHelper.FormatHelp)
	flags.VarP(&options.filter, "filter", "f", `Provide filter values (e.g. "driver=bridge")`)

	return cmd
}

func runList(ctx context.Context, dockerCli command.Cli, options listOptions) error {
	client := dockerCli.Client()
	networkResources, err := client.NetworkList(ctx, network.ListOptions{Filters: options.filter.Value()})
	if err != nil {
		return err
	}

	format := options.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().NetworksFormat) > 0 && !options.quiet {
			format = dockerCli.ConfigFile().NetworksFormat
		} else {
			format = formatter.TableFormatKey
		}
	}

	sort.Slice(networkResources, func(i, j int) bool {
		return sortorder.NaturalLess(networkResources[i].Name, networkResources[j].Name)
	})

	networksCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: NewFormat(format, options.quiet),
		Trunc:  !options.noTrunc,
	}
	return FormatWrite(networksCtx, networkResources)
}
