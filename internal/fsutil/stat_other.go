//go:build !darwin

package fsutil

import (
	"os"
	"time"
)

func Identity(_ os.FileInfo) (device, inode uint64, created *time.Time) { return 0, 0, nil }
