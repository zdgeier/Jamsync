package jam

import (
	"fmt"

	"github.com/zdgeier/jamhub/internal/jam/authfile"
)

func Logout() {
	authfile.Logout()
	fmt.Println("~/.jamhubauth file removed. Run `jam login` to log in.")
}
