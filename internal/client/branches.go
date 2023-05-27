package client

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	"github.com/zdgeier/jamsync/internal/server/server"
	"github.com/zdgeier/jamsync/internal/statefile"
	"golang.org/x/oauth2"
)

func ListBranches() {
	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	apiClient, closer, err := server.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
	})
	if err != nil {
		panic(err)
	}
	defer closer()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err != nil {
		panic(err)
	}

	state, err := statefile.Find()
	if err != nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
		os.Exit(0)
	}

	resp, err := apiClient.ListBranches(ctx, &pb.ListBranchesRequest{ProjectId: state.ProjectId})
	if err != nil {
		log.Panic(err)
	}

	for name := range resp.GetBranches() {
		fmt.Println(name)
	}
}
