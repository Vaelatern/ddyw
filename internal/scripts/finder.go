package scripts

import (
	"io/fs"
	"slices"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// From the README
//
// 2. Scripts/programs found in the --exec-dir directory
// 3. Scripts/programs found in the compile time embedded directory
// 4. Scripts/programs found at the --http-script-server site
// 5. Scripts/programs found in the compile time embedded directory, suffixed with `.fallback`
//
// Scripts are found depending on the mode of the agent. The agent can have a Role, it can be a Host Agent, it can be both, or it can be neither of these things. When searching for scripts, 2 through 5 are executed as follows:
//
// 1. `%s.RoleScriptName` - both Host and Role are set
// 2. `%s.ScriptName` - Host is set
// 3. `RoleScriptName - Role is set
// 4. `ScriptName`

type checkargs struct {
	fsys fs.FS
	name string
}

// Resolve is a longer function than it necessarily needs to be
// It's all in one so the details of execution can be altered later
// We need to go through all the possible names, given a name, and
// find the one that will match what we need.
func (r ResolutionContext) Resolve(name string) fs.File {
	// Basic lookups per the readme
	checkSets := []checkargs{
		checkargs{
			fsys: r.LocalDir,
			name: name,
		},
		checkargs{
			fsys: r.EmbeddedDir,
			name: name,
		},
		checkargs{
			fsys: r.RemoteDir,
			name: name,
		},
		checkargs{
			fsys: r.EmbeddedDir,
			name: name + ".fallback",
		},
	}

	// Construct a set of name alterations, in the form of functions to translate names
	nameAdjuster := []func(string) string{
		func(name string) string { // identity
			return name
		},
	}
	{
		isHost := r.Host != ""
		isRole := r.Role != ""
		normalizedRole := cases.Title(language.Und).String(r.Role)
		if isRole {
			nameAdjuster = append(nameAdjuster, func(name string) string {
				return normalizedRole + name
			})
		}
		if isHost {
			nameAdjuster = append(nameAdjuster, func(name string) string {
				return r.Host + "." + name
			})
		}
		if isHost && isRole {
			nameAdjuster = append(nameAdjuster, func(name string) string {
				return r.Host + "." + normalizedRole + name
			})
		}
		slices.Reverse(nameAdjuster)
	}

	// Expand the lists of possible scripts, building a whole matrix
	expandedCheckSets := []checkargs{}
	for _, check := range checkSets {
		for _, namefn := range nameAdjuster {
			expandedCheckSets = append(expandedCheckSets,
				checkargs{
					fsys: check.fsys,
					name: namefn(check.name),
				})
		}
	}

	// Check through the lists now that we know
	for _, set := range expandedCheckSets {
		if set.fsys == nil {
			continue
		}
		if rV := r.CheckName(set.fsys, set.name); rV != nil {
			return rV
		}
	}
	return nil
}

func (r ResolutionContext) CheckName(fsys fs.FS, name string) fs.File {
	file, err := fsys.Open(name)
	if err != nil {
		return nil
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil
	}
	if info.IsDir() {
		file.Close()
		return nil
	}
	return file
}
