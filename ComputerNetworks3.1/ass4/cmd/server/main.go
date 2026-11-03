package main

import (
	"ass4/server"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	port := flag.Int("port", 8080, "TCP port to listen on")
	numStations := flag.Int("n", 2, "Expected number of CDMA stations")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	srv, err := server.NewChannelServer(addr, *numStations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived shutdown signal. Closing server...")
		srv.Close()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server exited: %v\n", err)
		os.Exit(1)
	}
}
