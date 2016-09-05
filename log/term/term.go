package term

import (
	"io"
	"syscall"
	"unsafe"
	//	"fmt"
	//	"os"
)

type Termios syscall.Termios

type fder interface {
	Fd() uintptr
}

// IsTty returns true if the given file descriptor is a terminal.
func IsTty(w io.Writer) bool {
	fw, ok := w.(fder)
	if !ok {
		return false
	}
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fw.Fd(), ioctlReadTermios, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	//	if err != 0 { fmt.Println(err)}
	return err == 0
}

//// IsTty returns true if the given file descriptor is a terminal.
//func IsTty(fd uintptr) bool {
//        var termios Termios
//        _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioctlReadTermios, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
//        return err == 0
//}
