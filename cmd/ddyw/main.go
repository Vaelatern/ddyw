package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/Vaelatern/ddyw/internal/scripts"
	"github.com/Vaelatern/ddyw/internal/temporal"
	"github.com/hairyhenderson/go-fsimpl"
	"github.com/hairyhenderson/go-fsimpl/blobfs"
	"github.com/hairyhenderson/go-fsimpl/filefs"
	"github.com/hairyhenderson/go-fsimpl/gitfs"
	"github.com/hairyhenderson/go-fsimpl/httpfs"
)

//go:embed exec
var embeddedExecutables embed.FS

type AgentConfiguration struct {
	LocalConfig   interface{}
	ScriptContext scripts.ResolutionContext
}

type Config struct {
	Agent         AgentConfiguration
	execlocal     string
	execremote    string
	role          string
	taskprefix    string
	taskseparator string

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
	flag.StringVar(&rV.execlocal, "exec-local", "./exec/", "Directory for execution fallback")
	flag.StringVar(&rV.execremote, "exec-remote", "", "Remote findable execution location")
	flag.StringVar(&rV.taskprefix, "task-prefix", "!ddyw", "Temporal task queues begin with this prefix")
	flag.StringVar(&rV.taskseparator, "task-separator", "!", "Temporal task queue sections (preix, role, host) have this between them")

	// Parse flags
	flag.Parse()
	return rV
}

func computeFs(path string) (fs.FS, error) {
	if path == "" {
		return nil, nil
	}
	if strings.HasPrefix(path, "./") {
		cwd, err := os.Getwd()
		if err == nil {
			path = "file://" + cwd + "/" + path
		} else {
			return nil, fmt.Errorf("Current Working Directory for path \"%s\" failed: %v", path, err)
		}
	} else if strings.HasPrefix(path, "/") {
		path = "file://" + path
	}
	pathclone := path

	mux := fsimpl.NewMux()
	mux.Add(filefs.FS)
	mux.Add(httpfs.FS)
	mux.Add(blobfs.FS)
	mux.Add(gitfs.FS)
	fsys, err := mux.Lookup(pathclone)
	if err != nil {
		return nil, fmt.Errorf("Can't grab fsimpl filesystem \"%s\": %v", pathclone, err)
	}
	return fsys, nil
}

func bakeConfig() Config {
	rV := parseFlags()
	var err error

	if rV.hostagent {
		rV.Agent.ScriptContext.Host, err = os.Hostname()
		if err != nil {
			log.Fatalf("Failed to get hostname but was told to be a hostagent: %v", err)
		}
	}
	if rV.role != "" {
		rV.Agent.ScriptContext.Role = rV.role
	}
	rV.Agent.ScriptContext.EmbeddedDir, err = fs.Sub(embeddedExecutables, "exec")
	if err != nil {
		log.Fatalf("Failed to compute Embedded Execution Directory: %v", err)
	}
	rV.Agent.ScriptContext.LocalDir, err = computeFs(rV.execlocal)
	if err != nil {
		log.Fatalf("Failed to compute Local Execution Directory: %v", err)
	}
	rV.Agent.ScriptContext.RemoteDir, err = computeFs(rV.execremote)
	if err != nil {
		log.Fatalf("Failed to compute Remote Execution Directory: %v", err)
	}

	return rV
}

func (a *AgentConfiguration) DynAct(ctx context.Context, args converter.EncodedValues) (interface{}, error) {
	var some interface{}
	_ = args.Get(&some)
	info := activity.GetInfo(ctx)

	file := a.ScriptContext.Resolve(info.ActivityType.Name)
	if file == nil {
		return nil, fmt.Errorf("Failed to resolve %s", info.ActivityType.Name)
	}
	defer file.Close()
	return scripts.RunViaJson[interface{}](ctx, a.LocalConfig, some, file)

}

func Wflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second})
	err := workflow.ExecuteActivity(ctx, "Echo", struct {
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
	config := bakeConfig()

	taskQueue := config.taskprefix + config.taskseparator // default: !ddyw!
	if config.role != "" {
		taskQueue += "role" + config.taskseparator + config.role // role!RoleName
	}
	if config.hostagent {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		taskQueue += "host" + config.taskseparator + hostname // host!example.com
	}

	c, err := temporal.EasyClient(temporal.Logger())
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	var a AgentConfiguration = config.Agent

	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterWorkflow(Wflow)
	w.RegisterDynamicActivity(a.DynAct, activity.DynamicRegisterOptions{})

	// Start listening to the task queue
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatal(err)
	}
}
