package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/zdgeier/jamsync/gen/pb"
	serverclient "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func Merge() {
	config, _ := findJamsyncConfig()
	if config == nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
		return
	}

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

	_, err = apiClient.Ping(context.Background(), &pb.PingRequest{})
	if err != nil {
		loginAuth()
	}
	client := serverclient.NewClient(apiClient, config.ProjectId, config.BranchId)

	fileMetadata := readLocalFileList()
	remoteToLocalDiff, err := client.DiffRemoteToLocal(context.Background(), fileMetadata)
	if err != nil {
		log.Panic(err)
	}

	if diffHasChanges(remoteToLocalDiff) {
		fmt.Println("You currently have active changes. Run `jam push` to push your local changes.")
		return
	}

	_, err = apiClient.MergeBranch(context.Background(), &pb.MergeBranchRequest{
		ProjectId: config.GetProjectId(),
		BranchId:  config.GetBranchId(),
	})
	if err != nil {
		log.Panic(err)
	}
}
