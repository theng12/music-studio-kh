package process

import "syscall"

// stopWithParent kills a tool if the server is killed without a clean shutdown.
func stopWithParent(attr *syscall.SysProcAttr) { attr.Pdeathsig = syscall.SIGKILL }
