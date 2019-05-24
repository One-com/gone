// +build !go1.11

package sd

import (
	"os"
	unix "syscall"
)

// The Go (<1.11) net package sets the socket blocking.
func nonblockHack(file *os.File) error {
	return unix.SetNonblock(int(file.Fd()), true)
}
