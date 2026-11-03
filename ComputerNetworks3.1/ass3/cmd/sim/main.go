package main

import (
	"ass3/sim"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	mode := flag.String("mode", "csma-cd", "MAC Mode: non-persistent | 1-persistent | p-persistent | csma-cd")
	modesStr := flag.String("modes", "", "Comma-separated list of modes for heterogeneous stations (e.g. 'csma-cd,1-persistent,non-persistent,p-persistent')")
	pVal := flag.Float64("p", 0.5, "Probability p for p-persistent CSMA")
	stations := flag.Int("stations", 4, "Number of concurrent stations")
	frames := flag.Int("frames", 30, "Number of frames per station")
	tpStr := flag.String("tp", "5ms", "Channel propagation delay")
	tfrStr := flag.String("tfr", "10ms", "Frame transmission duration")
	quiet := flag.Bool("quiet", false, "Quiet mode (suppress per-frame logs)")
	csvPath := flag.String("csv", "eval/results.csv", "Path to CSV file to append summary metrics")
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

	var modes []string
	if *modesStr != "" {
		for _, m := range strings.Split(*modesStr, ",") {
			trimmed := strings.TrimSpace(m)
			if trimmed != "" {
				modes = append(modes, trimmed)
			}
		}
	}

	numStations := *stations
	if len(modes) > 0 && numStations < len(modes) {
		numStations = len(modes)
	}

	cfg := sim.RunConfig{
		Mode:        *mode,
		Modes:       modes,
		PVal:        *pVal,
		NumStations: numStations,
		Frames:      *frames,
		Tp:          tp,
		Tfr:         tfr,
		Quiet:       *quiet,
		CSVPath:     *csvPath,
	}

	if len(modes) > 0 {
		fmt.Printf("Starting Heterogeneous Simulation: Modes=%v, Stations=%d, Frames/Station=%d, Tp=%v, Tfr=%v...\n",
			modes, numStations, *frames, tp, tfr)
	} else {
		fmt.Printf("Starting Orchestrated Simulation: Mode=%s, Stations=%d, Frames/Station=%d, Tp=%v, Tfr=%v...\n",
			*mode, *stations, *frames, tp, tfr)
	}

	metrics, err := sim.RunSimulation(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Simulation failed: %v\n", err)
		os.Exit(1)
	}

	metrics.PrintSummary()
	if *csvPath != "" {
		fmt.Printf("Summary row appended to %s\n", *csvPath)
	}
}
