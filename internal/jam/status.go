package jam

import (
	"context"
	"fmt"
	"log"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jam/authfile"
	"github.com/zdgeier/jamhub/internal/jam/statefile"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

func Status() {
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

	nameResp, err := apiClient.GetProjectName(context.Background(), &pb.GetProjectNameRequest{
		ProjectId: state.ProjectId,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Project: %s\n", nameResp.ProjectName)

	if state.BranchInfo != nil {
		branchNameResp, err := apiClient.GetBranchName(context.Background(), &pb.GetBranchNameRequest{
			ProjectId: state.ProjectId,
			BranchId:  state.BranchInfo.BranchId,
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf(
			"Branch:  %s\n"+
				"Change:  %d\n",
			branchNameResp.GetBranchName(),
			state.BranchInfo.ChangeId,
		)

		changeResp, err := apiClient.GetBranchCurrentChange(context.Background(), &pb.GetBranchCurrentChangeRequest{ProjectId: state.ProjectId, BranchId: state.BranchInfo.BranchId})
		if err != nil {
			panic(err)
		}

		if changeResp.ChangeId == state.BranchInfo.ChangeId {
			fileMetadata := ReadLocalFileList()
			localToRemoteDiff, err := DiffLocalToRemoteBranch(apiClient, state.ProjectId, state.BranchInfo.BranchId, state.BranchInfo.ChangeId, fileMetadata)
			if err != nil {
				log.Panic(err)
			}

			if DiffHasChanges(localToRemoteDiff) {
				fmt.Println("\nModified files:")
				for path, diff := range localToRemoteDiff.Diffs {
					if diff.Type != pb.FileMetadataDiff_NoOp {
						fmt.Println("  " + path)
					}
				}
			} else {
				fmt.Println("\nNo local or remote changes.")

			}
		} else if changeResp.ChangeId > state.BranchInfo.ChangeId {
			fileMetadata := ReadLocalFileList()
			remoteToLocalDiff, err := DiffRemoteToLocalBranch(apiClient, state.ProjectId, state.BranchInfo.BranchId, state.BranchInfo.ChangeId, fileMetadata)
			if err != nil {
				log.Panic(err)
			}

			for path := range remoteToLocalDiff.GetDiffs() {
				fmt.Println(path, "changed")
			}
		} else {
			log.Panic("invalid state: local change id greater than remote change id")
		}
	} else {
		fmt.Printf("Commit:  %d\n", state.CommitInfo.CommitId)
	}
}
