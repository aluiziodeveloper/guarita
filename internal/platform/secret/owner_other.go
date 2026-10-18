//go:build !unix

package secret

import "os"

// owner não sabe o dono fora do Unix: o grupo nunca pode ler.
func owner(os.FileInfo) fileOwner { return fileOwner{} }
