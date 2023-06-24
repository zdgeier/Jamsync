package jam

import (
	"github.com/zdgeier/jamhub/internal/jam/authfile"
)

func Login() {
	_, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}
}
