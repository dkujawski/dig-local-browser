//go:build darwin

package fsutil

import (
	"os"
	"syscall"
	"time"
)

func Identity(info os.FileInfo) (device, inode uint64, created *time.Time) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, nil
	}
	t := time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
	return uint64(stat.Dev), stat.Ino, &t
}
