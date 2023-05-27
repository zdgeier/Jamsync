package client

import (
	"context"
	"fmt"
	"log"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	"github.com/zdgeier/jamsync/internal/server/server"
	"github.com/zdgeier/jamsync/internal/statefile"
	"golang.org/x/oauth2"
)

func Merge() {
	state, err := statefile.Find()
	if err != nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
		return
	}

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

	fileMetadata := readLocalFileList()
	remoteToLocalDiff, err := diffRemoteToLocal(apiClient, state.ProjectId, state.BranchId, state.ChangeId, fileMetadata)
	if err != nil {
		log.Panic(err)
	}

	if diffHasChanges(remoteToLocalDiff) {
		fmt.Println("You currently have active changes. Run `jam push` to push your local changes.")
		return
	}

	_, err = apiClient.MergeBranch(context.Background(), &pb.MergeBranchRequest{
		ProjectId: state.ProjectId,
		BranchId:  state.BranchId,
	})
	if err != nil {
		log.Panic(err)
	}
}
