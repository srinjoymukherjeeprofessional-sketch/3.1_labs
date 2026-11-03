package sim

import (
	"ass3/common"
	"ass3/server"
	"ass3/station"
	"context"
	"fmt"
	"sync"
	"time"
)

// RunConfig contains parameters for an orchestrated multi-station simulation.
type RunConfig struct {
	Mode        string        // Default or single MAC Mode: "non-persistent", "1-persistent", "p-persistent", "csma-cd"
	Modes       []string      // Optional list of heterogeneous modes (e.g. ["csma-cd", "1-persistent", ...])
	PVal        float64       // For p-persistent
	NumStations int           // Number of competing stations
	Frames      int           // Number of frames per station
	Tp          time.Duration // Propagation delay (vulnerability window)
	Tfr         time.Duration // Frame transmission time
	Quiet       bool          // Suppress per-frame console prints
	CSVPath     string        // Path to results.csv (empty to skip file write)
}

// RunSimulation orchestrates a complete simulation run.
func RunSimulation(cfg RunConfig) (common.SimulationMetrics, error) {
	if cfg.Tp <= 0 {
		cfg.Tp = common.DefaultTp
	}
	if cfg.Tfr <= 0 {
		cfg.Tfr = common.DefaultTfr
	}
	if len(cfg.Modes) > 0 && cfg.NumStations < len(cfg.Modes) {
		cfg.NumStations = len(cfg.Modes)
	}
	if cfg.NumStations <= 0 {
		cfg.NumStations = 4
	}
	if cfg.Frames <= 0 {
		cfg.Frames = 30
	}

	// 1. Start Channel Server on dynamic loopback port
	srv := server.NewServer("127.0.0.1:0", cfg.Tp, cfg.Quiet)
	if err := srv.Start(); err != nil {
		return common.SimulationMetrics{}, fmt.Errorf("orchestrator: failed to start server: %w", err)
	}
	defer srv.Close()

	serverAddr := srv.Addr()

	// 2. Initialize N stations
	stations := make([]*station.Station, cfg.NumStations)
	for i := 0; i < cfg.NumStations; i++ {
		stnMode := cfg.Mode
		if len(cfg.Modes) > 0 {
			stnMode = cfg.Modes[i%len(cfg.Modes)]
		}
		stn := station.NewStation(station.Config{
			ID:          uint16(i + 1),
			ServerAddr:  serverAddr,
			Mode:        stnMode,
			PVal:        cfg.PVal,
			TotalFrames: cfg.Frames,
			Tp:          cfg.Tp,
			Tfr:         cfg.Tfr,
			Quiet:       cfg.Quiet,
		})
		if err := stn.Connect(); err != nil {
			return common.SimulationMetrics{}, fmt.Errorf("orchestrator: failed to connect station %d: %w", i+1, err)
		}
		defer stn.Close()
		stations[i] = stn
	}

	// 3. Coordinate synchronized launch of all stations
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	startBarrier := make(chan struct{})
	stationErrors := make([]error, cfg.NumStations)

	for i := 0; i < cfg.NumStations; i++ {
		wg.Add(1)
		go func(idx int, stn *station.Station) {
			defer wg.Done()
			// Wait for barrier release so all stations begin contending concurrently
			<-startBarrier
			if err := stn.Run(ctx); err != nil && err != context.Canceled {
				stationErrors[idx] = err
			}
		}(i, stations[i])
	}

	// Release all stations simultaneously!
	simStart := time.Now()
	close(startBarrier)

	// Wait for all stations to complete their frame quotas
	wg.Wait()
	simDuration := time.Since(simStart)

	// 4. Check for any fatal station errors
	for i, err := range stationErrors {
		if err != nil {
			return common.SimulationMetrics{}, fmt.Errorf("orchestrator: station %d encountered error: %w", i+1, err)
		}
	}

	// 5. Aggregate metrics
	runMode := cfg.Mode
	if len(cfg.Modes) > 0 {
		runMode = "mixed"
	}
	metrics := srv.Channel().SnapshotMetrics(runMode, cfg.NumStations, cfg.Tfr, cfg.PVal)
	metrics.Duration = simDuration // Use exact client-observed duration
	metrics.PerStation = make(map[uint16]*common.StationMetrics)

	totalDropped := 0
	var totalDelay time.Duration
	for _, stn := range stations {
		sm := stn.Metrics()
		metrics.PerStation[stn.ID] = &sm
		totalDropped += sm.FramesDropped
		totalDelay += sm.TotalDelay
	}
	metrics.TotalDropped = totalDropped
	if metrics.TotalAttempts > 0 {
		metrics.AvgDelayMs = float64(totalDelay.Microseconds()) / (1000.0 * float64(metrics.TotalAttempts))
	}
	metrics.ComputeDerivedMetrics()

	// 6. Write to CSV if configured
	if cfg.CSVPath != "" {
		_ = metrics.AppendToCSV(cfg.CSVPath)
	}

	return metrics, nil
}
