package main

import (
	"flag"
	_ "net/http/pprof"
	"os"

	"github.com/zdgeier/jamsync/internal/client"
)

var (
	version string
	built   string
)

func main() {
	flag.Parse()

	switch {
	case len(os.Args) == 1:
		client.Help(version, built)
	case os.Args[1] == "login":
		client.Login()
	case os.Args[1] == "init":
		client.InitConfig()
	case os.Args[1] == "open":
		client.Open()
	case os.Args[1] == "pull":
		client.Pull()
	case os.Args[1] == "status":
		client.Status()
	case os.Args[1] == "push":
		client.Push()
	case os.Args[1] == "merge":
		client.Merge()
	case os.Args[1] == "checkout":
		client.Checkout()
	case os.Args[1] == "branches":
		client.ListBranches()
	case os.Args[1] == "projects":
		client.ListProjects()
	case os.Args[1] == "logout":
		client.Logout()
	case os.Args[1] == "delete":
		client.Delete()
	default:
		client.Help(version, built)
	}
}
