// +build !arm

package main

import (
	"syscall"
	"time"
)

func setSysTime(t time.Time) {
	ns := t.UnixNano()
	s := ns / 1e9
	us := (ns - s*1e9) * 1e3
	tv := syscall.Timeval{s, us}
	fatalIf(syscall.Settimeofday(&tv))
}
