package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift-online/rosa-hyperfleet-zoa/internal/client"
	"github.com/openshift-online/rosa-hyperfleet-zoa/internal/output"
)

func newTargetsCommand(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "targets <deployment>",
		Short: "List targets within a deployment",
		Long: `List the targets (RC and MC clusters) available in a given deployment.

Use a deployment name from 'zoa deployments' (e.g. us-east-1). The CLI
auto-resolves the Access Lambda and invoker role from SSM, then lists targets.

STATUS is registration metadata from cluster Terraform (ready = registered in SSM
with boundary VPC/ECS metadata and eligible for session start; not a health probe).

Default table shows TARGET, REGION, TYPE, and STATUS. Use -o wide for full SSM
metadata (VPC, subnets, ECS, Function URL, etc.); use -o json for automation.`,
		Example: `  zoa targets us-east-1

  zoa targets us-east-1 -o wide

  zoa targets us-east-1 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deployment := args[0]
			if opts.APIURL == "" {
				opts.Deployment = deployment
			}

			c, err := getClient(opts)
			if err != nil {
				return fmt.Errorf("creating client: %w", err)
			}

			list, err := c.ListTargets(cmd.Context())
			if err != nil {
				return fmt.Errorf("listing targets: %w", err)
			}

			if opts.OutputFormat == output.FormatJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(list)
			}

			if len(list.Items) == 0 {
				fmt.Printf("No targets found for deployment %q\n", deployment)
				return nil
			}

			return printTargetsTable(os.Stdout, opts, list.Items)
		},
	}

	return cmd
}

func printTargetsTable(w io.Writer, opts *GlobalOptions, items []client.Target) error {
	wide := opts.OutputFormat == output.FormatWide
	tw := output.NewTable(w)
	if wide {
		fmt.Fprintln(tw, "TARGET\tDEPLOYMENT\tREGION\tTYPE\tSTATUS\tACCOUNT\tVPC\tSUBNETS\tSECURITY GROUP\tECS CLUSTER\tTASK DEFINITION\tFUNCTION URL")
	} else {
		fmt.Fprintln(tw, "TARGET\tREGION\tTYPE\tSTATUS")
	}
	for _, t := range items {
		if wide {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				t.TargetID,
				output.Dash(t.DeploymentName),
				output.Dash(t.Region),
				t.TargetType,
				output.Dash(t.Status),
				output.Dash(t.AccountId),
				output.Dash(t.VpcId),
				output.Dash(t.SubnetIds),
				output.Dash(t.SecurityGroupId),
				output.Dash(t.EcsClusterArn),
				output.Dash(t.TaskDefinitionArn),
				output.Dash(t.FunctionUrl))
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			t.TargetID, output.Dash(t.Region), t.TargetType, output.Dash(t.Status))
	}
	return tw.Flush()
}
