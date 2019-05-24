// +build go1.11

package sd

import (
	"os"
)

// The Go (<1.11) net package sets the socket blocking.
// No longer necessary per Go 1.11 release notes.
func nonblockHack(file *os.File) error {
	return nil
}
