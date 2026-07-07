// Package status provides a lightweight, non-persisted way to check whether
// a host is currently reachable — a plain TCP dial, nothing stored.
package status

import (
	"fmt"
	"net"
	"time"
)

// Probe reports whether a TCP connection to address:port succeeds within
// timeout. This is a reachability check, not an SSH handshake — cheap
// enough to run repeatedly against many hosts without much overhead.
func Probe(address string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
