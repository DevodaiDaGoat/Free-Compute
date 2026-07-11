//go:build windows

package main

import (
	"net"
)

func setTCPKeepaliveAggressive(conn *net.TCPConn) error {
	return nil
}