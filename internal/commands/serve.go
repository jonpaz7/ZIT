package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"zit/internal/peer"
	"zit/internal/repo"
	"zit/internal/tracker"
)

func Serve(trackerURL, peerAddr string) error {
	r, err := repo.Find(".")
	if err != nil {
		return err
	}

	if trackerURL == "" {
		trackerURL = r.Config.Tracker
	}
	if peerAddr == "" {
		peerAddr = r.Config.Peer
	}
	if peerAddr == "" {
		peerAddr = "127.0.0.1:8001"
	}

	var trackerCli *tracker.TrackerClient
	if trackerURL != "" {
		trackerCli = tracker.NewClient(trackerURL)
	}

	srv := peer.NewServer(peerAddr, r.LocalStore(), trackerCli)

	objects, _ := r.LocalStore().List()

	fmt.Printf("Starting ZIT peer server on %s\n", peerAddr)
	if trackerURL != "" {
		fmt.Printf("Connected tracker: %s\n", trackerURL)
	}
	fmt.Printf("Local objects ready to serve: %d\n", len(objects))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down peer server...")
		cancel()
	}()

	return srv.Start(ctx)
}
