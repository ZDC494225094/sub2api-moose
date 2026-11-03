//go:build !linux && !darwin && !freebsd && !windows

package gatewaylogarchive

func diskFree(string) int64 { return -1 }
