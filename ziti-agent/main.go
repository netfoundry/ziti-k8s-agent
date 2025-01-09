package main

import (
	"flag"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/netfoundry/ziti-k8s-agent/ziti-agent/cmd"
)

func main() {
	// add klog flags to the standard flag set
	klog.InitFlags(flag.CommandLine)
	
	// Add the command line flags from pflags to the standard flag set
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	
	// Set default log level (this can be overridden by command line)
	_ = pflag.Set("v", "2") // Set default log level to INFO
	
	cmd.Execute()
}
