package client

import (
	"github.com/zdgeier/jamsync/internal/authfile"
)

func Login() {
	_, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}
}
