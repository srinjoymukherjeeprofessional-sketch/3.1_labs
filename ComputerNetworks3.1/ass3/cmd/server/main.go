package main

import (
	"ass3/common"
	"ass3/server"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	port := flag.Int("port", 8080, "TCP port to listen on")
	tpStr := flag.String("tp", "5ms", "One-way channel propagation delay (e.g. 5ms)")
	quiet := flag.Bool("quiet", false, "Suppress per-event terminal logs")
	flag.Parse()

	tp, err := time.ParseDuration(*tpStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid -tp duration: %v\n", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", *port)
	srv := server.NewServer(addr, tp, *quiet)

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║               CHANNEL SERVER RUNNING (IEEE 802.3 CSMA)            ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("  Listening on Address   : %s\n", srv.Addr())
	fmt.Printf("  Propagation Delay (Tp) : %v\n", tp)
	fmt.Printf("  Slot Time (2*Tp)       : %v\n", 2*tp)
	fmt.Println("  Ready for concurrent station connections... Press Ctrl+C to stop.")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nReceived shutdown signal. Generating channel metrics snapshot...")
	srv.Close()

	metrics := srv.Channel().SnapshotMetrics("standalone", 0, common.DefaultTfr, 0)
	metrics.PrintSummary()
}
