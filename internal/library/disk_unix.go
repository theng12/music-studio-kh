//go:build !windows

package library

import "syscall"

// volumeSpace reports capacity and free bytes of the filesystem holding path
// with one statfs call — no directory walk.
//
// shortcut: uses f_bsize (equal to f_frsize on mainstream Linux filesystems);
// switch to f_frsize if a filesystem ever reports them differently.
func volumeSpace(path string) (total, free uint64, err error) {
	var st syscall.Statfs_t
	if err = syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bsize := uint64(st.Bsize)
	return st.Blocks * bsize, st.Bavail * bsize, nil
}
