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
		fmt.Println("Could not find a `.jam` file. Run `jam init` to initialize the project.")
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
		fmt.Println("Currently on a commit, workon a workspace with `jam workon <workspacename>` to push changes.")
		os.Exit(1)
	}

	fileMetadata := ReadLocalFileList()
	remoteToLocalDiff, err := DiffRemoteToLocalWorkspace(apiClient, state.ProjectId, state.WorkspaceInfo.WorkspaceId, state.WorkspaceInfo.ChangeId, fileMetadata)
	if err != nil {
		log.Panic(err)
	}

	if DiffHasChanges(remoteToLocalDiff) {
		fmt.Println("You currently have active changes. Run `jam push` to push your local changes.")
		return
	}

	resp, err := apiClient.MergeWorkspace(context.Background(), &pb.MergeWorkspaceRequest{
		ProjectId:   state.ProjectId,
		WorkspaceId: state.WorkspaceInfo.WorkspaceId,
	})
	if err != nil {
		log.Panic(err)
	}

	_, err = apiClient.DeleteWorkspace(context.Background(), &pb.DeleteWorkspaceRequest{
		ProjectId:   state.ProjectId,
		WorkspaceId: state.WorkspaceInfo.WorkspaceId,
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
