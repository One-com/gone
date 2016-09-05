// +build darwin

package term

import "syscall"

const ioctlReadTermios = syscall.TIOCGETA
