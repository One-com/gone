package pool

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// channelPool implements the Pool interface based on buffered channels.
type channelPool struct {
	mu sync.Mutex

	// storage for idle net.Conn connections
	conns chan net.Conn

	maxconns  int
	openconns int
	blocking  bool

	// net.Conn generator
	factory Factory
}

// Factory is a function to create new connections.
type Factory func() (net.Conn, error)

// NewChannelPool returns a new pool based on buffered channels with an idle
// capacity and maximum capacity. Factory is used when initial capacity is
// greater than zero to fill the pool. If there is no new connection
// available in the pool, a new connection will be created via the Factory()
// method.
// If blocking is true, Get() block until there's a connection available when the pool is full
// If blocking is false, Get() returns an error.
func NewChannelPool(idleSize int, maxSize int, factory Factory, blocking bool) (Pool, error) {
	if idleSize < 0 || maxSize <= 0 || idleSize >= maxSize {
		return nil, errors.New("invalid capacity settings")
	}

	c := &channelPool{
		conns:    make(chan net.Conn, idleSize),
		factory:  factory,
		maxconns: maxSize,
		blocking: blocking,
	}

	return c, nil
}

// Get implements the Pool interfaces Get() method. If there is no new
// connection available in the pool, a new connection will be created via the
// Factory() method. The boolean return parameter indicated if it's a fresh minted connection.
func (c *channelPool) Get() (*PoolConn, bool, error) {

	// wrap our connections with out custom net.Conn implementation (wrapConn
	// method) that puts the connection back to the pool if it's closed.
	// We try to get a connection from the idle queue without locking the pool.
	// If that fails we have to lock and do some book-keeping with connection, - maybe
	// create a new one.
	// However... if the pool is "full" and we are configured to block, then we release
	// the pool and sit down to wait for someone to pul a used connection into the idle queue.
	select {
	case conn := <-c.conns:
		if conn == nil {
			return nil, false, ErrClosed
		}
		return c.wrapConn(conn), false, nil
	default:
		c.mu.Lock()
		if c.openconns >= c.maxconns {
			c.mu.Unlock()
			if c.blocking {
				// wait for a connection
				conn := <-c.conns
				if conn == nil {
					return nil, false, ErrClosed
				}
				return c.wrapConn(conn), false, nil
			} else {
				return nil, false, errors.New("Connection pool full")
			}
		}
		conn, err := c.factory()
		if err != nil {
			c.mu.Unlock()
			return nil, false, err
		}
		c.openconns++
		c.mu.Unlock()
		return c.wrapConn(conn), true, nil
	}
}

// Let the pool know that we are closing a connetion. To keep track of open connections.
func (c *channelPool) closeConn(conn net.Conn) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.openconns--
	return conn.Close()
}

// put puts the connection back to the pool. If the pool is full or closed,
// conn is simply closed. A nil conn will be rejected.
func (c *channelPool) putConn(conn net.Conn) error {
	if conn == nil {
		return errors.New("connection is nil. rejecting")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.factory == nil {
		// pool is closed, close passed connection
		c.openconns--
		return conn.Close()
	}

	// Disable timeout while the conn is in the queue
	conn.SetDeadline(time.Time{})

	// put the resource back into the pool. If the pool is full, this will
	// block and the default case will be executed.
	select {
	case c.conns <- conn:
		return nil
	default:
		// pool is full, close passed connection
		c.openconns--
		return conn.Close()
	}
}

func (c *channelPool) Close() {
	c.mu.Lock()

	for conn := range c.conns {
		c.openconns--
		conn.Close()
	}
	close(c.conns)
	c.factory = nil // mark the pool as closed.
	c.mu.Unlock()
}
