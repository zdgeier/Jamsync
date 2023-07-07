package main

import (
	"flag"
	_ "net/http/pprof"
	"os"

	"github.com/zdgeier/jamhub/internal/jam"
)

var (
	version string
	built   string
)

func main() {
	flag.Parse()

	switch {
	case len(os.Args) == 1:
		jam.Help(version, built)
	case os.Args[1] == "login":
		jam.Login()
	case os.Args[1] == "init":
		jam.InitConfig()
	case os.Args[1] == "open":
		jam.Open()
	case os.Args[1] == "pull":
		jam.Pull()
	case os.Args[1] == "status":
		jam.Status()
	case os.Args[1] == "push":
		jam.Push()
	case os.Args[1] == "merge":
		jam.Merge()
	case os.Args[1] == "workon":
		jam.WorkOn()
	case os.Args[1] == "workspaces":
		jam.ListWorkspaces()
	case os.Args[1] == "projects":
		jam.ListProjects()
	case os.Args[1] == "logout":
		jam.Logout()
	case os.Args[1] == "delete":
		jam.Delete()
	default:
		jam.Help(version, built)
	}
}
