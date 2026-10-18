//go:build unix

package secret

import (
	"os"
	"syscall"
)

// #region secret-owner-unix
// owner lê o dono e o grupo do stat do sistema. Fora do Unix, o
// arquivo owner_other.go diz que não se sabe, e só o modo vale.
func owner(info os.FileInfo) fileOwner {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileOwner{}
	}
	return fileOwner{known: true, uid: int(st.Uid), gid: int(st.Gid)}
}

// #endregion
