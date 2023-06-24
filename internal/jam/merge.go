package jam

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jam/authfile"
	"github.com/zdgeier/jamhub/internal/jam/statefile"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

func Merge() {
	state, err := statefile.Find()
	if err != nil {
		fmt.Println("Could not find a `.jamhub` file. Run `jam init` to initialize the project.")
		return
	}

	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	apiClient, closer, err := jamhubgrpc.Connect(&oauth2.Token{
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
