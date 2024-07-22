package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/netfoundry/ziti-k8s-agent/ziti-agent/cmd/common"
	"github.com/netfoundry/ziti-k8s-agent/ziti-agent/cmd/webhook"
	"github.com/spf13/cobra"
)

type RootCmd struct {
	cobraCommand *cobra.Command
}

func GetRootCommand() *cobra.Command {
	return rootCommand.cobraCommand
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "\n%v\n", err)
	os.Exit(1)
}

var rootCommand = RootCmd{
	cobraCommand: &cobra.Command{
		Use:   "ziti-agent",
		Short: "ziti-agent is CLI for working with ziti k8s agent",
		Long: `
'ziti-agent' is CLI for working with ziti agent on kubernetes platforms.
`},
}

func Execute() {
	if err := rootCommand.cobraCommand.Execute(); err != nil {
		exitWithError(err)
	}
}

func init() {
	NewCmdRoot(os.Stdin, os.Stdout, os.Stderr, rootCommand.cobraCommand)
}

func NewCmdRoot(in io.Reader, out, err io.Writer, cmd *cobra.Command) *cobra.Command {

	cmd.AddCommand(webhook.NewWebhookCmd())
	cmd.AddCommand(common.NewVersionCmd())

	return cmd
}
