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

func Pull() {
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
		log.Panic(err)
	}
	defer closer()

	if state.CommitInfo == nil {
		changeResp, err := apiClient.GetBranchCurrentChange(context.Background(), &pb.GetBranchCurrentChangeRequest{ProjectId: state.ProjectId, BranchId: state.BranchInfo.BranchId})
		if err != nil {
			panic(err)
		}

		fileMetadata := readLocalFileList()
		remoteToLocalDiff, err := diffRemoteToLocalBranch(apiClient, state.ProjectId, state.BranchInfo.BranchId, changeResp.GetChangeId(), fileMetadata)
		if err != nil {
			log.Panic(err)
		}

		if diffHasChanges(remoteToLocalDiff) {
			err = applyFileListDiffBranch(apiClient, state.ProjectId, state.BranchInfo.BranchId, changeResp.GetChangeId(), remoteToLocalDiff)
			if err != nil {
				log.Panic(err)
			}
			for key, val := range remoteToLocalDiff.GetDiffs() {
				if val.Type != pb.FileMetadataDiff_NoOp {
					fmt.Println("Pulled", key)
				}
			}
		} else {
			fmt.Println("No changes to pull")
		}
		err = statefile.StateFile{
			ProjectId: state.ProjectId,
			BranchInfo: &statefile.BranchInfo{
				BranchId: state.BranchInfo.BranchId,
				ChangeId: changeResp.ChangeId,
			},
		}.Save()
		if err != nil {
			panic(err)
		}
	} else {
		commitResp, err := apiClient.GetProjectCurrentCommit(context.Background(), &pb.GetProjectCurrentCommitRequest{ProjectId: state.ProjectId})
		if err != nil {
			panic(err)
		}

		fileMetadata := readLocalFileList()
		remoteToLocalDiff, err := diffRemoteToLocalCommit(apiClient, state.ProjectId, commitResp.CommitId, fileMetadata)
		if err != nil {
			log.Panic(err)
		}

		if diffHasChanges(remoteToLocalDiff) {
			err = applyFileListDiffCommit(apiClient, state.ProjectId, commitResp.CommitId, remoteToLocalDiff)
			if err != nil {
				log.Panic(err)
			}
			for key, val := range remoteToLocalDiff.GetDiffs() {
				if val.Type != pb.FileMetadataDiff_NoOp {
					fmt.Println("Pulled", key)
				}
			}
		} else {
			fmt.Println("No commits to pull")
		}

		err = statefile.StateFile{
			ProjectId: state.ProjectId,
			CommitInfo: &statefile.CommitInfo{
				CommitId: commitResp.CommitId,
			},
		}.Save()
		if err != nil {
			panic(err)
		}
	}
}
