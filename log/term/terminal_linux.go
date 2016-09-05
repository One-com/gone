// +build linux

package term

import "syscall"

const ioctlReadTermios = syscall.TCGETS
