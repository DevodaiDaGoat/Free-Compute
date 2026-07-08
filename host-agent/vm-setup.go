package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/freecompute/free-compute/host-agent/internal/vmagent"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "print the resolved VM agent config and routes, then exit without launching")
	selfTest := flag.Bool("self-test", false, "register with the gateway and emit one metrics report, then exit")
	flag.Parse()

	config, routes, gatewayURL, token := vmagent.LoadVMConfig()

	if *dryRun {
		vmagent.PrintDryRun(config, routes, gatewayURL, token)
		return
	}

	agent := vmagent.NewVMAgent(config, gatewayURL, token, routes)

	if *selfTest {
		if err := agent.SelfTest(context.Background()); err != nil {
			log.Printf("self-test error: %v", err)
			os.Exit(1)
		}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := agent.Start(ctx); err != nil {
		log.Fatalf("failed to start VM agent: %v", err)
	}

	// Wait for context cancellation
	<-ctx.Done()
}
