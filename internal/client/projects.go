package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func ListProjects() {
	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	apiClient, closer, err := server.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
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
