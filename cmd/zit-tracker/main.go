package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"zit/internal/tracker"
)

func main() {
	port := flag.Int("port", 9000, "Port to listen on")
	dumpFile := flag.String("dump", "tracker_data.json", "JSON dump file for persisting tracker state")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	srv := tracker.NewServer(*dumpFile)

	log.Printf("Starting ZIT Tracker on %s (dump: %s)...", addr, *dumpFile)
	log.Printf("Endpoints:")
	log.Printf("  POST /announce")
	log.Printf("  GET  /providers/{hash}")
	log.Printf("  GET  /refs/{repoId}")
	log.Printf("  PUT  /refs/{repoId}/{ref}")

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("Tracker server failed: %v", err)
	}
}
