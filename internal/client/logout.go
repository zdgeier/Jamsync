package client

import (
	"fmt"
	"log"
	"os"
)

func Logout() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Panic(err)
	}
	err = os.Remove(authPath(home))
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("~/.jamsyncauth file removed. Run `jam login` to log in.")
}
