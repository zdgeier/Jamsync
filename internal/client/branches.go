package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zdgeier/jamsync/gen/pb"
	serverclient "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func ListBranches() {
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

	config, _ := findJamsyncConfig()
	if config == nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
		return
	}

	client := serverclient.NewClient(apiClient, config.ProjectId, config.BranchId)

	resp, err := apiClient.ListBranches(ctx, &pb.ListBranchesRequest{ProjectId: client.ProjectConfig().ProjectId})
	if err != nil {
		log.Panic(err)
	}

	for name := range resp.GetBranches() {
		fmt.Println(name)
	}
}
