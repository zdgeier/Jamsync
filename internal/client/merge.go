package client

import (
	"context"
	"fmt"
	"log"
	"os"

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

	if state.CommitInfo != nil {
		fmt.Println("Currently on a commit, checkout a branch with `jam checkout <branchname>` to push changes.")
		os.Exit(1)
	}

	fileMetadata := ReadLocalFileList()
	remoteToLocalDiff, err := DiffRemoteToLocalBranch(apiClient, state.ProjectId, state.BranchInfo.BranchId, state.BranchInfo.ChangeId, fileMetadata)
	if err != nil {
		log.Panic(err)
	}

	if DiffHasChanges(remoteToLocalDiff) {
		fmt.Println("You currently have active changes. Run `jam push` to push your local changes.")
		return
	}

	resp, err := apiClient.MergeBranch(context.Background(), &pb.MergeBranchRequest{
		ProjectId: state.ProjectId,
		BranchId:  state.BranchInfo.BranchId,
	})
	if err != nil {
		log.Panic(err)
	}

	_, err = apiClient.DeleteBranch(context.Background(), &pb.DeleteBranchRequest{
		ProjectId: state.ProjectId,
		BranchId:  state.BranchInfo.BranchId,
	})
	if err != nil {
		log.Panic(err)
	}

	err = statefile.StateFile{
		ProjectId: state.ProjectId,
		CommitInfo: &statefile.CommitInfo{
			CommitId: resp.CommitId,
		},
	}.Save()
	if err != nil {
		log.Panic(err)
	}
}
