package common

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// ─── StationMetrics ───────────────────────────────────────────────────────────

// StationMetrics records per-station transmission outcomes.
type StationMetrics struct {
	StationID        uint16
	Mode             string
	FramesAttempted  int
	FramesSuccessful int
	Collisions       int
	FramesDropped    int
	TotalWaitTime    time.Duration
	TotalTxTime      time.Duration
	TotalDelay       time.Duration // Time spent contending + transmitting each frame
}

// ─── SimulationMetrics ────────────────────────────────────────────────────────

// SimulationMetrics aggregates global network statistics for a simulation run.
type SimulationMetrics struct {
	Mode             string
	PValue           float64 // Only relevant for p-persistent
	NumStations      int
	Tp               time.Duration
	Tfr              time.Duration
	SlotTime         time.Duration
	Duration         time.Duration
	TotalAttempts    int
	TotalSuccess     int
	TotalCollisions  int
	TotalDropped     int
	TotalPayloadBits int64
	ThroughputKbps   float64
	EfficiencyPct    float64
	DeliveryRatioPct float64
	AvgDelayMs       float64
	PerStation       map[uint16]*StationMetrics
}

// ComputeDerivedMetrics computes throughput, efficiency, and delivery ratio.
func (m *SimulationMetrics) ComputeDerivedMetrics() {
	if m.Duration.Seconds() > 0 {
		m.ThroughputKbps = float64(m.TotalPayloadBits) / (m.Duration.Seconds() * 1000.0)
		totalTxSuccessful := time.Duration(m.TotalSuccess) * m.Tfr
		m.EfficiencyPct = (float64(totalTxSuccessful) / float64(m.Duration)) * 100.0
	}
	if totalUnique := m.TotalSuccess + m.TotalDropped; totalUnique > 0 {
		m.DeliveryRatioPct = (float64(m.TotalSuccess) / float64(totalUnique)) * 100.0
	} else if m.TotalAttempts > 0 {
		m.DeliveryRatioPct = (float64(m.TotalSuccess) / float64(m.TotalAttempts)) * 100.0
	}
}

// PrintSummary outputs a structured ASCII summary block.
func (m *SimulationMetrics) PrintSummary() {
	fmt.Printf("\n╔═══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║                     CSMA / CSMA-CD SIMULATION SUMMARY             ║\n")
	fmt.Printf("╠═══════════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("  MAC Mode              : %s", m.Mode)
	if m.Mode == "p-persistent" {
		fmt.Printf(" (p = %.2f)\n", m.PValue)
	} else {
		fmt.Printf("\n")
	}
	fmt.Printf("  Active Stations       : %d\n", m.NumStations)
	fmt.Printf("  Channel Propagation Tp: %v\n", m.Tp)
	fmt.Printf("  Frame Tx Time Tfr     : %v\n", m.Tfr)
	fmt.Printf("  Slot Time (2*Tp)      : %v\n", m.SlotTime)
	fmt.Printf("  Wall Clock Duration   : %v\n", m.Duration.Round(time.Millisecond))
	fmt.Printf("───────────────────────────────────────────────────────────────────\n")
	fmt.Printf("  Total Frame Attempts  : %d\n", m.TotalAttempts)
	fmt.Printf("  Successful Deliveries : %d\n", m.TotalSuccess)
	fmt.Printf("  Collision Events      : %d\n", m.TotalCollisions)
	fmt.Printf("  Frames Dropped        : %d\n", m.TotalDropped)
	fmt.Printf("───────────────────────────────────────────────────────────────────\n")
	fmt.Printf("  Throughput            : %.2f kbps\n", m.ThroughputKbps)
	fmt.Printf("  Channel Efficiency    : %.2f%%\n", m.EfficiencyPct)
	fmt.Printf("  Delivery Ratio (PDR)  : %.2f%%\n", m.DeliveryRatioPct)
	fmt.Printf("  Avg Frame Delay       : %.2f ms\n", m.AvgDelayMs)
	fmt.Printf("╚═══════════════════════════════════════════════════════════════════╝\n")

	if len(m.PerStation) > 1 {
		fmt.Printf("\n┌──────────┬──────────────────┬──────────┬──────────┬────────────┬─────────┬──────────┐\n")
		fmt.Printf("│ Station  │ MAC Mode         │ Attempts │ Delivery │ Collisions │ Dropped │ PDR (%%)  │\n")
		fmt.Printf("├──────────┼──────────────────┼──────────┼──────────┼────────────┼─────────┼──────────┤\n")
		for id := uint16(1); id <= uint16(len(m.PerStation)); id++ {
			sm, ok := m.PerStation[id]
			if !ok {
				continue
			}
			pdr := 0.0
			if sm.FramesAttempted > 0 {
				pdr = (float64(sm.FramesSuccessful) / float64(sm.FramesAttempted)) * 100.0
			}
			fmt.Printf("│ Stn #%-3d │ %-16s │ %-8d │ %-8d │ %-10d │ %-7d │ %6.1f%%  │\n",
				sm.StationID, sm.Mode, sm.FramesAttempted, sm.FramesSuccessful, sm.Collisions, sm.FramesDropped, pdr)
		}
		fmt.Printf("└──────────┴──────────────────┴──────────┴──────────┴────────────┴─────────┴──────────┘\n\n")
	}
}

var csvMutex sync.Mutex

// AppendToCSV writes simulation metrics to a CSV file.
func (m *SimulationMetrics) AppendToCSV(filePath string) error {
	csvMutex.Lock()
	defer csvMutex.Unlock()

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	writeHeader := false
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeHeader = true
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if writeHeader {
		header := []string{
			"mode",
			"p_val",
			"num_stations",
			"tp_ms",
			"tfr_ms",
			"duration_ms",
			"total_attempts",
			"total_success",
			"total_collisions",
			"total_dropped",
			"throughput_kbps",
			"efficiency_pct",
			"delivery_ratio_pct",
			"avg_delay_ms",
		}
		if err := w.Write(header); err != nil {
			return err
		}
	}

	row := []string{
		m.Mode,
		fmt.Sprintf("%.2f", m.PValue),
		strconv.Itoa(m.NumStations),
		fmt.Sprintf("%.2f", float64(m.Tp.Microseconds())/1000.0),
		fmt.Sprintf("%.2f", float64(m.Tfr.Microseconds())/1000.0),
		fmt.Sprintf("%.2f", float64(m.Duration.Microseconds())/1000.0),
		strconv.Itoa(m.TotalAttempts),
		strconv.Itoa(m.TotalSuccess),
		strconv.Itoa(m.TotalCollisions),
		strconv.Itoa(m.TotalDropped),
		fmt.Sprintf("%.4f", m.ThroughputKbps),
		fmt.Sprintf("%.2f", m.EfficiencyPct),
		fmt.Sprintf("%.2f", m.DeliveryRatioPct),
		fmt.Sprintf("%.2f", m.AvgDelayMs),
	}

	return w.Write(row)
}
