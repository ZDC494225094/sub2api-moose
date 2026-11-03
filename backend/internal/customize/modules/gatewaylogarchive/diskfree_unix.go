//go:build linux || darwin || freebsd

package gatewaylogarchive

import "syscall"

// diskFree returns the bytes available to unprivileged users, or -1.
func diskFree(dir string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return -1
	}
	return int64(uint64(st.Bavail) * uint64(st.Bsize))
}
