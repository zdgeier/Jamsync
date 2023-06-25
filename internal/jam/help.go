package jam

import (
	"fmt"
	"os"

	"github.com/zdgeier/jamhub/internal/jamenv"
)

func Help(version string, built string) {
	fmt.Println("Welcome to JamHub!")
	fmt.Println("\nversion:", version)
	fmt.Println("built:  ", built)
	fmt.Println("env:    ", jamenv.Env().String())
	fmt.Println("\nlogin    - do this first. creates ~/.jamhubauth.")
	fmt.Println("init     - initialize a project in the current directory.")
	fmt.Println("open     - open the current project in the browser.")
	fmt.Println("status   - print information about the local state of the project.")
	fmt.Println("push     - push up local modifications to a workspace.")
	fmt.Println("pull     - pull down remote modifications to the mainline or workspace.")
	fmt.Println("checkout - create or download a workspace.")
	fmt.Println("workspaces - list active workspaces.")
	fmt.Println("projects - list your projects.")
	fmt.Println("logout   - deletes ~/.jamhubauth.")
	fmt.Println("delete   - delete the project in the current directory or by name.")
	fmt.Println("help     - show this text")
	fmt.Println("\nHappy jammin'!")
	os.Exit(0)
}
