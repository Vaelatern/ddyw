package main

import (
	"context"
	"flag"
	"log"
	"os"

	"go.temporal.io/sdk/worker"

	"github.com/Vaelatern/ddyw/internal/temporal"
)

type AgentConfiguration struct{}

type Config struct {
	ExecOnDisk string
	role       string

	hostagent bool
	file      string
	dir       string
}

func parseFlags() Config {
	rV := Config{}

	flag.StringVar(&rV.file, "config", "config.toml", "Path to a single config file")
	flag.StringVar(&rV.dir, "config-dir", "config.d", "Path to a directory of config files")
	flag.BoolVar(&rV.hostagent, "host-agent", false, "Enable host mode")
	flag.StringVar(&rV.role, "role", "", "Take on a specific role")
	flag.StringVar(&rV.ExecOnDisk, "exec-dir", "./exec/", "Directory for execution fallback")

	// Parse flags
	flag.Parse()
	return rV
}

func (a *AgentConfiguration) DynAct(ctx context.Context) error {
	return nil
}

func main() {
	config := parseFlags()

	taskQueue := "!ddyw!"
	if config.role != "" {
		taskQueue += "role!" + config.role
	}
	if config.hostagent {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		taskQueue += "host!" + hostname
	}

	c, err := temporal.EasyClient(temporal.Logger())
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	a := AgentConfiguration{}

	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterDynamicActivity(a.DynAct)

	// Start listening to the task queue
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatal(err)
	}
}
