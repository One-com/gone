package vtransport

import (
	"io"
)

type deferCloseBody struct {
	io.ReadCloser
	isread      bool
	shouldClose bool
}

func (c *deferCloseBody) Read(bs []byte) (int, error) {
	if !c.isread {
		c.isread = true
	}
	if c.ReadCloser == nil {
		return 0, io.EOF
	}
	return c.ReadCloser.Read(bs)
}

// A special Close() to not close request bodies, if they have not been read
// but only remember to do so later. - so requests can be retried
func (c *deferCloseBody) Close() error {

	if !c.shouldClose && !c.isread {
		c.shouldClose = true
		return nil
	}
	c.shouldClose = false // don't close twice
	if c.ReadCloser != nil {
		return c.ReadCloser.Close()
	}
	return nil
}

func (c *deferCloseBody) CanRetry() bool {
	return !c.isread
}

func (c *deferCloseBody) CloseIfNeeded() error {
	if c.shouldClose {
		if c.ReadCloser != nil {
			return c.ReadCloser.Close()
		}
	}
	return nil
}
