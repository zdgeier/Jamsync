package jam

import (
	"fmt"
	"log"
	"os"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jam/authfile"
	"github.com/zdgeier/jamhub/internal/jam/statefile"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

func Push() {
	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	stateFile, err := statefile.Find()
	if err != nil {
		fmt.Println("Could not find a `.jam` file. Run `jam init` to initialize the project.")
		os.Exit(1)
	}

	apiClient, closer, err := jamhubgrpc.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	if stateFile.CommitInfo != nil {
		fmt.Println("Currently on a commit, workon a workspace with `jam workon <workspacename>` to push changes.")
		os.Exit(1)
	}

	fileMetadata := ReadLocalFileList()
	localToRemoteDiff, err := DiffLocalToRemoteWorkspace(apiClient, stateFile.ProjectId, stateFile.WorkspaceInfo.WorkspaceId, stateFile.WorkspaceInfo.ChangeId, fileMetadata)
	if err != nil {
		log.Panic(err)
	}

	changeId := stateFile.WorkspaceInfo.ChangeId + 1
	if DiffHasChanges(localToRemoteDiff) {
		err = pushFileListDiffWorkspace(apiClient, stateFile.ProjectId, stateFile.WorkspaceInfo.WorkspaceId, changeId, fileMetadata, localToRemoteDiff)
		if err != nil {
			log.Panic(err)
		}
		for key, val := range localToRemoteDiff.GetDiffs() {
			if val.Type != pb.FileMetadataDiff_NoOp {
				fmt.Println("Pushed", key)
			}
		}
	} else {
		fmt.Println("No changes to push")
	}

	err = statefile.StateFile{
		ProjectId: stateFile.ProjectId,
		WorkspaceInfo: &statefile.WorkspaceInfo{
			WorkspaceId: stateFile.WorkspaceInfo.WorkspaceId,
			ChangeId:    changeId,
		},
	}.Save()
	if err != nil {
		panic(err)
	}
}
