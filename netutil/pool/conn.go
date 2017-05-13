package pool

import "net"

// poolConn is a wrapper around net.Conn to modify the the behavior of
// net.Conn's Close() method.
type PoolConn struct {
	net.Conn
	pool *channelPool
}

// Release() puts the net.Conn back to the pool instead of closing it.
func (pc PoolConn) Release() error {
	return pc.pool.putConn(pc.Conn)
}

// Close() closes the net.Conn and discards it as broken.
func (pc PoolConn) Close() error {
	return pc.pool.closeConn(pc.Conn)
}

// newConn wraps a standard net.Conn to a PoolConn net.Conn.
func (c *channelPool) wrapConn(conn net.Conn) *PoolConn {
	p := PoolConn{Conn:conn, pool: c}
	return &p
}
