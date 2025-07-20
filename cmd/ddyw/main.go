package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/Vaelatern/ddyw/internal/temporal"
)

type AgentConfiguration struct {
	LocalConfig interface{}
}

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

func (a *AgentConfiguration) DynAct(ctx context.Context, args converter.EncodedValues) error {
	var some interface{}
	_ = args.Get(&some)

	wrapped := struct {
		Local  interface{}
		Passed interface{}
	}{
		Local:  a.LocalConfig,
		Passed: some,
	}

	body, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	os.Stdout.Write(body)
	return nil
}

func Wflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second})
	err := workflow.ExecuteActivity(ctx, "random-activity-name", struct {
		Abc string
		Omg int
	}{
		Abc: "Hello",
		Omg: 11111,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}
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
	w.RegisterWorkflow(Wflow)
	w.RegisterDynamicActivity(a.DynAct, activity.DynamicRegisterOptions{})

	// Start listening to the task queue
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatal(err)
	}
}
