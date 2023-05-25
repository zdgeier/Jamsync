package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func ListProjects() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Panic(err)
	}
	accessToken, err := os.ReadFile(authPath(home))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("Run `jam login` to login to Jamsync first (" + home + "/.jamsyncauth does not exist).")
		os.Exit(1)
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err != nil {
		log.Panic(err)
	}

	resp, err := apiClient.ListUserProjects(ctx, &pb.ListUserProjectsRequest{})
	if err != nil {
		log.Panic(err)
	}

	for _, proj := range resp.GetProjects() {
		fmt.Println(proj.GetName())
	}
}
