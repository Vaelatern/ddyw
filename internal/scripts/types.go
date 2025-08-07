package scripts

import "io/fs"

type ResolutionContext struct {
	LocalDir    fs.FS
	RemoteDir   fs.FS
	EmbeddedDir fs.FS
	Host        string
	Role        string
}
