package common

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version will be injected at build time
var Version = "v0.0.0"

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show agent version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(Version)
		},
	}
}
