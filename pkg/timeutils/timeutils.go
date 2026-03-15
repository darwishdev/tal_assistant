package timeutils

import (
	"fmt"
	"time"
)

// MsToSRT converts a millisecond offset to SRT timestamp format (HH:MM:SS,mmm).
func MsToSRT(ms int64) string {
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	s := (ms % 60000) / 1000
	f := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, f)
}

func NowMs() int64 {
	return time.Now().UnixMilli()
}
