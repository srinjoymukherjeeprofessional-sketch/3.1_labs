package main

import (
	"flag"
	"fmt"
	"ass4/station"
)

func main() {
	serverAddr := flag.String("server", "localhost:8080", "Channel Server address")
	flag.Parse()

	fmt.Printf("Station client targeted at %s\n", *serverAddr)
	
	stn := station.NewStation(*serverAddr)
	if err := stn.Connect(); err != nil {
		fmt.Printf("Connection error: %v\n", err)
		return
	}
	defer stn.Close()

	if err := stn.RunInteractiveLoop(); err != nil {
		fmt.Printf("Station stopped: %v\n", err)
	}
	
}
