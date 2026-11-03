package main

import (
	"ass3/station"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	id := flag.Int("id", 1, "Station ID")
	serverAddr := flag.String("server", "localhost:8080", "Channel Server address")
	mode := flag.String("mode", "csma-cd", "MAC Mode: non-persistent | 1-persistent | p-persistent | csma-cd")
	pVal := flag.Float64("p", 0.5, "Probability p for p-persistent CSMA")
	frames := flag.Int("frames", 20, "Number of frames to transmit")
	tpStr := flag.String("tp", "5ms", "Channel propagation delay")
	tfrStr := flag.String("tfr", "10ms", "Frame transmission duration")
	quiet := flag.Bool("quiet", false, "Quiet mode (suppress per-frame stdout logs)")
	flag.Parse()

	tp, err := time.ParseDuration(*tpStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid -tp duration: %v\n", err)
		os.Exit(1)
	}

	tfr, err := time.ParseDuration(*tfrStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid -tfr duration: %v\n", err)
		os.Exit(1)
	}

	stn := station.NewStation(station.Config{
		ID:          uint16(*id),
		ServerAddr:  *serverAddr,
		Mode:        *mode,
		PVal:        *pVal,
		TotalFrames: *frames,
		Tp:          tp,
		Tfr:         tfr,
		Quiet:       *quiet,
	})

	if err := stn.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Connection failed: %v\n", err)
		os.Exit(1)
	}
	defer stn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	fmt.Printf("[Station %2d] Connected to server %s. Running mode: %s (%d frames)...\n",
		*id, *serverAddr, *mode, *frames)

	start := time.Now()
	err = stn.Run(ctx)
	elapsed := time.Since(start)

	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "[Station %2d] Error: %v\n", *id, err)
		os.Exit(1)
	}

	m := stn.Metrics()
	fmt.Printf("\n─── Station %d Summary ──────────────────────\n", *id)
	fmt.Printf("  Mode             : %s\n", *mode)
	fmt.Printf("  Frames Attempted : %d\n", m.FramesAttempted)
	fmt.Printf("  Frames Succeeded : %d\n", m.FramesSuccessful)
	fmt.Printf("  Collisions       : %d\n", m.Collisions)
	fmt.Printf("  Frames Dropped   : %d\n", m.FramesDropped)
	fmt.Printf("  Elapsed Time     : %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("─────────────────────────────────────────────\n")
}
