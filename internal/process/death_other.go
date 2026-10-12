//go:build !linux && !windows

package process

import "syscall"

// Other Unix systems have no parent-death signal; clean shutdown stops tools.
func stopWithParent(*syscall.SysProcAttr) {}
