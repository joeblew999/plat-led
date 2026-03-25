// LED Client - Control client for led-server
// Connects through ssh-bridge local forward to reach led-server
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
	serverAddr = flag.String("server", "localhost:8002", "address of led-server (via ssh tunnel)")
)

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("led-client connecting to %s", *serverAddr)

	conn, err := net.Dial("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	log.Printf("connected to led-server")

	// Start reader goroutine
	done := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			fmt.Printf("< %s\n", scanner.Text())
		}
		close(done)
	}()

	// Read commands from stdin
	fmt.Println("Commands: ping, status, set <r> <g> <b>, quit")
	fmt.Println("---")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fmt.Fprintln(conn, line)

		if line == "quit" {
			break
		}
	}

	<-done
	log.Printf("disconnected")
}
