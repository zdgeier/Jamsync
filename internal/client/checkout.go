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

func Checkout() {
	if len(os.Args) != 3 {
		fmt.Println("jam checkout <branch name>")
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

	if state.CommitInfo == nil || state.BranchInfo != nil {
		fmt.Println("Must be on mainline to checkout.")
		os.Exit(1)
	}

	resp, err := apiClient.ListBranches(ctx, &pb.ListBranchesRequest{ProjectId: state.ProjectId})
	if err != nil {
		panic(err)
	}

	if branchId, ok := resp.GetBranches()[os.Args[2]]; ok {
		if branchId == state.BranchInfo.BranchId {
			fmt.Println("Already on", os.Args[2])
			return
		}

		changeResp, err := apiClient.GetBranchCurrentChange(context.TODO(), &pb.GetBranchCurrentChangeRequest{ProjectId: state.ProjectId, BranchId: branchId})
		if err != nil {
			panic(err)
		}

		// if branch already exists, do a pull
		fileMetadata := readLocalFileList()
		remoteToLocalDiff, err := diffRemoteToLocalBranch(apiClient, state.ProjectId, state.BranchInfo.BranchId, changeResp.ChangeId, fileMetadata)
		if err != nil {
			log.Panic(err)
		}

		if diffHasChanges(remoteToLocalDiff) {
			err = applyFileListDiffBranch(apiClient, state.ProjectId, state.BranchInfo.BranchId, changeResp.ChangeId, remoteToLocalDiff)
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
		// otherwise, just create a new branch
		resp, err := apiClient.CreateBranch(ctx, &pb.CreateBranchRequest{ProjectId: state.ProjectId, BranchName: os.Args[2]})
		if err != nil {
			log.Panic(err)
		}

		fmt.Println(resp.BranchId)
		err = statefile.StateFile{
			ProjectId: state.ProjectId,
			BranchInfo: &statefile.BranchInfo{
				BranchId: resp.BranchId,
			},
		}.Save()
		if err != nil {
			panic(err)
		}
		fmt.Println("Switched to new branch", os.Args[2], "with id", resp.GetBranchId())
	}
}
