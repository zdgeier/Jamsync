package jam

import (
	"github.com/zdgeier/jamhub/internal/jam/authfile"
)

func Login() {
	authfile.Logout()
	_, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}
}
