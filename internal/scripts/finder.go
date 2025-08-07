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

func (r ResolutionContext) Resolve(name string) fs.File {
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

	for _, set := range checkSets {
		if set.fsys == nil {
			continue
		}
		rV := r.CheckName(set.fsys, set.name)
		if rV != nil {
			return rV
		}
	}
	return nil
}

func (r ResolutionContext) CheckName(fsys fs.FS, name string) fs.File {
	isHost := r.Host != ""
	isRole := r.Role != ""
	normalizedRole := cases.Title(language.Und).String(r.Role)
	checkOrder := []string{name}
	if isRole {
		checkOrder = append(checkOrder, normalizedRole+name)
	}
	if isHost {
		checkOrder = append(checkOrder, r.Host+"."+name)
	}
	if isHost && isRole {
		checkOrder = append(checkOrder, r.Host+"."+normalizedRole+name)
	}

	slices.Reverse(checkOrder) // Other order please

	for _, script := range checkOrder {
		file, err := fsys.Open(script)
		if err != nil {
			continue
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			continue
		}
		if info.IsDir() {
			file.Close()
			continue
		}
		return file
	}
	return nil
}
