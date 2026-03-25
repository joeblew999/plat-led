// LED Server - Pico simulator for desktop testing
// Connects to ssh-bridge with a reverse tunnel, exposing a control port
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

var (
	listenAddr = flag.String("listen", ":8001", "address to listen for control connections")
)

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("led-server starting on %s", *listenAddr)

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	log.Printf("led-server ready, waiting for connections...")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()
	log.Printf("client connected: %s", remoteAddr)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		log.Printf("[%s] recv: %s", remoteAddr, line)

		// Simple command handling
		var response string
		switch {
		case line == "ping":
			response = "pong"
		case line == "status":
			response = "ok:led-server:sim"
		case strings.HasPrefix(line, "set "):
			// Simulate LED control: set <r> <g> <b>
			response = "ack:" + line
		case line == "quit":
			fmt.Fprintln(conn, "bye")
			log.Printf("[%s] disconnected (quit)", remoteAddr)
			return
		default:
			response = "err:unknown command"
		}

		fmt.Fprintln(conn, response)
		log.Printf("[%s] sent: %s", remoteAddr, response)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[%s] read error: %v", remoteAddr, err)
	} else {
		log.Printf("[%s] disconnected", remoteAddr)
	}
	os.Stdout.Sync()
}
