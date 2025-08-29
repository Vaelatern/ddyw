package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"

	"github.com/fsnotify/fsnotify"
	"github.com/hairyhenderson/go-fsimpl"
	"github.com/hairyhenderson/go-fsimpl/blobfs"
	"github.com/hairyhenderson/go-fsimpl/filefs"
	"github.com/hairyhenderson/go-fsimpl/gitfs"
	"github.com/hairyhenderson/go-fsimpl/httpfs"

	"github.com/Vaelatern/ddyw/internal/customization"
	"github.com/Vaelatern/ddyw/internal/scripts"
	"github.com/Vaelatern/ddyw/internal/temporal"
)

var DEFAULT_CONFIG_D string = "config.d"

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
	watchdir      string

	hostagent bool
	conffile  string
	confdir   string
}

func parseFlags() Config {
	rV := Config{}

	flag.StringVar(&rV.conffile, "config", "config.toml", "Path to a single config file")
	flag.StringVar(&rV.confdir, "config-dir", DEFAULT_CONFIG_D, "Path to a directory of config files")
	flag.BoolVar(&rV.hostagent, "host-agent", false, "Enable host mode")
	flag.StringVar(&rV.role, "role", "", "Take on a specific role")
	flag.StringVar(&rV.execlocal, "exec-local", "./exec/", "Directory for execution fallback")
	flag.StringVar(&rV.execremote, "exec-remote", "", "Remote findable execution location")
	flag.StringVar(&rV.taskprefix, "task-prefix", "!ddyw", "Temporal task queues begin with this prefix")
	flag.StringVar(&rV.taskseparator, "task-separator", "!", "Temporal task queue sections (preix, role, host) have this between them")
	flag.StringVar(&rV.watchdir, "watch-dir", "watch", "Watch this directory for json formatted activities to trigger directly")

	// Parse flags
	flag.Parse()
	return rV
}

func computeFs(path string) (fs.FS, error) {
	if path == "" {
		return nil, nil
	}
	if strings.HasPrefix(path, "./") || !strings.Contains(path, "://") {
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

	// Execution Directories
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

	//// Configuration Files
	conffile, err := os.Open(rV.conffile)
	var noconffile, noconfdir error
	if err != nil {
		noconffile = fmt.Errorf("Failed to compute Configuration File: %v", err)
	}
	confdir, err := computeFs(rV.confdir)
	if err != nil {
		noconfdir = fmt.Errorf("Failed to compute Configuration Dir: %v", err)
	}

	if noconffile != nil && noconfdir != nil {
		log.Fatalf("Can't compute conf file %s (%v) or dir %s (%v)", rV.conffile, noconffile, rV.confdir, noconfdir)
	} else if noconffile != nil {
		log.Printf("Not loading conf file %s: %v", rV.conffile, noconffile)
	} else if noconfdir != nil {
		log.Printf("Not loading conf dir %s: %v", rV.confdir, noconfdir)
	}

	var fileEntries []fs.File
	entries, err := fs.ReadDir(confdir, ".")
	if err != nil && rV.confdir != DEFAULT_CONFIG_D { // allow ignoring the default config directory
		log.Fatalf("Can't read directory %s: %v", confdir, err)
	}
	for _, f := range entries {
		fAsFile, err := confdir.Open(f.Name())
		if err != nil {
			log.Fatalf("Can't read conf file within directory: %s: %v", f.Name(), err)
		}
		fileEntries = append(fileEntries, fAsFile)
	}
	if conffile != nil {
		fileEntries = append(fileEntries, conffile)
	}
	rV.Agent.LocalConfig, err = DeepMergeFiles(fileEntries)
	if err != nil {
		log.Fatalf("Merge failed: %v", err)
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

func (c Config) runLocalCommandedScript(name string) func() {
	return func() {
		fp, err := os.Open(name)
		if err != nil {
			fmt.Printf("Failed opening script %s: %v\n", name, err)
			return
		}
		defer fp.Close()
		err = scripts.RunLocalViaJson(c.Agent.LocalConfig,
			c.Agent.ScriptContext,
			c.watchdir,
			fp)
		if err != nil {
			fmt.Printf("Failed running script %s: %v\n", name, err)
			return
		}
		fp.Close()
		err = os.Remove(name)
		if err != nil {
			fmt.Printf("Failed removing script %s after run: %v\n", name, err)
			return
		}
	}
}

func (c Config) listDir() <-chan string {
	rV := make(chan string)
	go func() {
		defer close(rV)
		dir, err := os.Open(c.watchdir)
		if err != nil {
			fmt.Printf("Failed to open watching dir to list items: %v\n", err)
			return
		}
		defer dir.Close()
		allNames, err := dir.Readdirnames(0)
		if err != nil {
			fmt.Printf("Failed to list watching dir items: %v\n", err)
			return
		}
		for _, name := range allNames {
			rV <- filepath.Join(c.watchdir, name)
		}
	}()
	return rV
}

func (c Config) watchAndProcDir() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("Failed to create watcher: %v", err)
	}
	defer w.Close()

	// No thanks, don't use this feature
	if c.watchdir == "" {
		return nil
	}

	// Get the dir, please.
	err = w.Add(c.watchdir)
	if err != nil {
		return fmt.Errorf("Failed to add dir '%s' to watcher: %v", c.watchdir, err)
	}

	var timers map[string]*time.Timer = make(map[string]*time.Timer)

	initialRead := c.listDir()
	debounce := 200 * time.Millisecond
	for {
		select {
		case fullName := <-initialRead:
			if !strings.HasSuffix(fullName, ".json.in") {
				continue
			}
			name := filepath.Base(fullName)
			if name == "." || name[0] == os.PathSeparator {
				continue
			}
			name = strings.TrimSuffix(name, ".json.in")
			// If the timer doesn't exist, or if it already expired, add the timer.
			if timers[name] == nil || timers[name].Reset(debounce) {
				timers[name] = time.AfterFunc(debounce, c.runLocalCommandedScript(fullName))
			}
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Write) && strings.HasSuffix(event.Name, ".json.in") {
				name := filepath.Base(event.Name)
				if name == "." || name[0] == os.PathSeparator {
					continue
				}
				name = strings.TrimSuffix(name, ".json.in")
				// If the timer doesn't exist, or if it already expired, add the timer.
				if timers[name] == nil || timers[name].Reset(debounce) {
					timers[name] = time.AfterFunc(debounce, c.runLocalCommandedScript(event.Name))
				}
			}
		case err := <-w.Errors:
			return err
		}
	}

	return nil
}

func main() {
	config := bakeConfig()

	taskQueue := config.taskprefix // default: !ddyw
	if config.role != "" {
		taskQueue += config.taskseparator + "role" + config.taskseparator + config.role // !role!RoleName
	}
	if config.hostagent {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		taskQueue += config.taskseparator + "host" + config.taskseparator + hostname // !host!example.com
	}

	c, err := temporal.EasyClient(temporal.Logger())
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	var a AgentConfiguration = config.Agent

	w := worker.New(c, taskQueue, worker.Options{})

	for i := range customization.Activities {
		w.RegisterActivityWithOptions(customization.Activities[i],
			customization.ActivityOptions[i])
	}
	for i := range customization.Workflows {
		w.RegisterWorkflowWithOptions(customization.Workflows[i],
			customization.WorkflowOptions[i])
	}

	w.RegisterDynamicActivity(a.DynAct, activity.DynamicRegisterOptions{})

	// Start listening to the task queue
	go func() {
		err := config.watchAndProcDir()
		if err != nil {
			log.Fatal(err)
		}
	}()
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatal(err)
	}
}
