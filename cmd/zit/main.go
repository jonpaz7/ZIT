package main

import (
	"flag"
	"fmt"
	"os"
	"zit/internal/commands"
)

const usage = `ZIT - Distributed Version Control System with Torrent-like Lazy Object Storage

Usage:
  zit <command> [arguments]

Local Git Commands:
  init [dir]               Initialize a new zit repository
  add <path...>            Stage files / directories for commit
  commit -m "message"      Create a new commit from staged files
  branch [name]            List branches or create a new branch
  checkout <branch|hash>   Switch branches or checkout files
  log                      Show commit logs
  status                   Show working tree and staging status

Network & P2P Commands:
  serve                    Run peer server to seed local objects to other clients
  publish                  Publish current branch and announce objects to tracker
  clone <tracker> <repoId> Clone repo metadata (refs, commits, trees) without blobs
  fetch                    Fetch latest refs, commits, and trees from tracker
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "init":
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		trackerURL := fs.String("tracker", "", "Tracker URL")
		peerAddr := fs.String("peer", "127.0.0.1:8001", "Peer address host:port")
		_ = fs.Parse(args)
		if err := commands.Init(fs.Args(), *trackerURL, *peerAddr); err != nil {
			fatal(err)
		}

	case "add":
		if len(args) == 0 {
			fatal(fmt.Errorf("usage: zit add <path...>"))
		}
		if err := commands.Add(args); err != nil {
			fatal(err)
		}

	case "commit":
		fs := flag.NewFlagSet("commit", flag.ExitOnError)
		msg := fs.String("m", "", "Commit message")
		author := fs.String("author", "", "Commit author")
		_ = fs.Parse(args)
		if *msg == "" {
			fatal(fmt.Errorf("commit message required (-m \"message\")"))
		}
		if err := commands.Commit(*msg, *author); err != nil {
			fatal(err)
		}

	case "branch":
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if err := commands.Branch(name); err != nil {
			fatal(err)
		}

	case "checkout":
		if err := commands.Checkout(args); err != nil {
			fatal(err)
		}

	case "log":
		if err := commands.Log(); err != nil {
			fatal(err)
		}

	case "status":
		if err := commands.Status(); err != nil {
			fatal(err)
		}

	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		trackerURL := fs.String("tracker", "", "Tracker URL")
		peerAddr := fs.String("peer", "", "Peer address host:port (or :port)")
		port := fs.Int("port", 0, "Peer port")
		_ = fs.Parse(args)
		addr := *peerAddr
		if *port != 0 {
			addr = fmt.Sprintf("127.0.0.1:%d", *port)
		}
		if err := commands.Serve(*trackerURL, addr); err != nil {
			fatal(err)
		}

	case "publish":
		if err := commands.Publish(); err != nil {
			fatal(err)
		}

	case "clone":
		fs := flag.NewFlagSet("clone", flag.ExitOnError)
		peerAddr := fs.String("peer", "127.0.0.1:8002", "Local peer address for serving")
		_ = fs.Parse(args)
		remArgs := fs.Args()
		if len(remArgs) < 2 {
			fatal(fmt.Errorf("usage: zit clone <tracker-url> <repoId> [dir]"))
		}
		destDir := ""
		if len(remArgs) > 2 {
			destDir = remArgs[2]
		}
		if err := commands.Clone(remArgs[0], remArgs[1], destDir, *peerAddr); err != nil {
			fatal(err)
		}

	case "fetch":
		if err := commands.Fetch(); err != nil {
			fatal(err)
		}

	case "help", "--help", "-h":
		fmt.Print(usage)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
