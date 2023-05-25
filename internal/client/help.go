package client

import (
	"fmt"
	"os"

	"github.com/zdgeier/jamsync/internal/jamenv"
)

func Help(version string, built string) {
	fmt.Println("Welcome to Jamsync!")
	fmt.Println("\nversion:", version)
	fmt.Println("built:  ", built)
	fmt.Println("env:    ", jamenv.Env().String())
	fmt.Println("\nlogin  - do this first. creates ~/.jamsyncauth.")
	fmt.Println("init   - initialize a project in the current directory.")
	fmt.Println("sync   - start syncing a project in the current directory.")
	fmt.Println("open   - open repository in the browser.")
	fmt.Println("delete - delete the project in the current directory or by name.")
	fmt.Println("logout - deletes ~/.jamsyncauth.")
	fmt.Println("help   - show this text")
	fmt.Println("\nHappy syncin'!")
	os.Exit(0)
}
