package main

import (
	"fmt"
	"os"

	"github.com/bzdvdn/maskchain/src/internal/infra/config"
	"github.com/bzdvdn/maskchain/src/pkg/version"
	"github.com/spf13/cobra"
)

func main() {
	cmd := config.NewGatewayRootCmd()
	cmd.Version = version.Info()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		run()
		return nil
	}
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
