# Durably Declare Your World

Ever run ansible against 99% of your fleet, and then had to chase down the last 1% to figure out why they didn't get the change?

Ever had salt apply against most of your systems but some systems were offline and never got the message?

What if the systems didn't have to be online when you ordered a change? What if your systems when they come back, or just exist, will get the change when they are able?

## Arguments

- `--exec-dir`: Path to a directory on the filesystem or a [go-fsimpl](https://github.com/hairyhenderson/go-fsimpl) compatible link using blobfs, filefs, gitfs, or httpfs. Scripts/executables/programs will be opened under this directory.
- `--http-script-server`: Link compatible with [go-fsimpl](https://github.com/hairyhenderson/go-fsimpl). Commands/executables will be found under this directory.
- `--task-queue-root`: Defaults to `!ddyw!`. Prefixes all task queues.
- `--role`: Appends `role!%s` to the task queue, where %s is this argument, allowing grouping agents by role. This is an arbitrary string. When role is present, scripts are found first by looking for `RoleScriptName` then `ScriptName` for each of the extensibility options.
- `--host-agent`: Sets host mode. Appends `host!%s` to the task queue, where %s is the detected hostname, allowing targetting specific hosts for specific work. When executing an ansible script on a host, you are thinking of executing on a host-agent. When host is present, scripts are found first by looking for `%s.ScriptName` then `ScriptName` for each of the extensibility options. When Role and Host are both found, the match is run for `%s.RoleScriptName` then `%s.ScriptName` then `RoleScriptName` then `ScriptName`.
- `--watch-dir`: It is useful to instruct the agent to do a command it knows without the Temporal apparatus involved. This allows a person to drop a `.json.in` file in the directory, and the file will be processed, moved to the `output/` directory within the watch-dir directory, prepended with the date, and a `.json.out` file placed with the JSON output of the command. The nesting of the directory as `output/` allows low numbers of file handles on the BSD and related platforms, and has no express benefit on Linux except cleanliness. The filename before `.json.in` will be the name of the executable script to use. An example watch dir including example output is provided in `examples/`

## Extending the agent

Routines are executed in the following order of precedence

1. Built in Golang functions directly registered as Temporal activities
2. Scripts/programs found in the --exec-dir directory
3. Scripts/programs found in the compile time embedded directory
4. Scripts/programs found at the --http-script-server site
5. Scripts/programs found in the compile time embedded directory, suffixed with `.fallback`

Convention is that the source chosen for a run will identify itself, to ease debugging, with one of the following notes `[exec:embedded]` `[exec:dir]` `[exec:dir-internal]` `[exec:http]`

Extensions are expected and you are expected to write a new one easily whenever you need a new capability.

Scripts are found depending on the mode of the agent. The agent can have a Role, it can be a Host Agent, it can be both, or it can be neither of these things. When searching for scripts, 2 through 5 are executed as follows:

1. `%s.RoleScriptName` - both Host and Role are set
2. `%s.ScriptName` - Host is set
3. `RoleScriptName - Role is set
4. `ScriptName`

There is no way to override a built-in Golang function. The name is always executed as exists, without prefixing the name at all.

Scripts are checked in the order of precedence first, and only within the order of precedence are they checked based on agent mode.

### Exec Dir vs. HTTP Script Server

Both of these take the same format of arguments so you can comfortably, for example, put an http server earlier in the search tree than the internal compiled directory. It is intended that you put the lower latency directory, or easier to override directory, earlier in the search tree, to aid with custom overrides during development or one-off issues in prod.
