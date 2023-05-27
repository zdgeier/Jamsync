package client

import (
	"fmt"

	"github.com/zdgeier/jamsync/internal/authfile"
)

func Logout() {
	authfile.Logout()
	fmt.Println("~/.jamsyncauth file removed. Run `jam login` to log in.")
}
