package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func Login() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Panic(err)
	}
	accessToken, err := os.ReadFile(authPath(home))
	if errors.Is(err, os.ErrNotExist) {
		loginAuth()
		return
	} else if err != nil {
		panic(err)
	}

	apiClient, closer, err := server.Connect(&oauth2.Token{
		AccessToken: string(accessToken),
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	pingRes, err := apiClient.Ping(context.Background(), &pb.PingRequest{})
	if err != nil {
		loginAuth()
	} else {
		fmt.Println("Already logged in as", pingRes.GetUsername(), ". Run `jam logout` to log out.")
	}
}
