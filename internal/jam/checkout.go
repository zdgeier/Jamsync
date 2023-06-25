package jam

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jam/authfile"
	"github.com/zdgeier/jamhub/internal/jam/statefile"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

func Checkout() {
	if len(os.Args) != 3 {
		fmt.Println("jam checkout <workspace name>")
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
		fmt.Println("Could not find a `.jamhub` file. Run `jam init` to initialize the project.")
		os.Exit(0)
	}

	if state.CommitInfo == nil || state.WorkspaceInfo != nil {
		if os.Args[2] == "main" || os.Args[2] == "mainline" {
			fileMetadata := ReadLocalFileList()
			localToRemoteDiff, err := DiffLocalToRemoteWorkspace(apiClient, state.ProjectId, state.WorkspaceInfo.WorkspaceId, state.WorkspaceInfo.ChangeId, fileMetadata)
			if err != nil {
				log.Panic(err)
			}
			if DiffHasChanges(localToRemoteDiff) {
				fmt.Println("Some changes locally have not been pushed. Run `jam push` to push your local changes.")
				os.Exit(1)
			}

			commitResp, err := apiClient.GetProjectCurrentCommit(context.Background(), &pb.GetProjectCurrentCommitRequest{
				ProjectId: state.ProjectId,
			})
			if err != nil {
				log.Panic(err)
			}

			diffRemoteToLocalResp, err := DiffRemoteToLocalCommit(apiClient, state.ProjectId, commitResp.CommitId, &pb.FileMetadata{})
			if err != nil {
				log.Panic(err)
			}

			err = ApplyFileListDiffCommit(apiClient, state.ProjectId, commitResp.CommitId, diffRemoteToLocalResp)
			if err != nil {
				log.Panic(err)
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
			return
		} else {
			fmt.Println("Must be on mainline to checkout.")
			os.Exit(1)
		}
	}

	if os.Args[2] == "main" || os.Args[2] == "mainline" {
		fmt.Println("`main` and `mainline` are workspace names reserved for commits. Please choose another workspace name.")
		os.Exit(1)
	}

	resp, err := apiClient.ListWorkspaces(ctx, &pb.ListWorkspacesRequest{ProjectId: state.ProjectId})
	if err != nil {
		panic(err)
	}

	if workspaceId, ok := resp.GetWorkspaces()[os.Args[2]]; ok {
		if workspaceId == state.WorkspaceInfo.WorkspaceId {
			fmt.Println("Already on", os.Args[2])
			return
		}

		changeResp, err := apiClient.GetWorkspaceCurrentChange(context.TODO(), &pb.GetWorkspaceCurrentChangeRequest{ProjectId: state.ProjectId, WorkspaceId: workspaceId})
		if err != nil {
			panic(err)
		}

		// if workspace already exists, do a pull
		fileMetadata := ReadLocalFileList()
		remoteToLocalDiff, err := DiffRemoteToLocalWorkspace(apiClient, state.ProjectId, state.WorkspaceInfo.WorkspaceId, changeResp.ChangeId, fileMetadata)
		if err != nil {
			log.Panic(err)
		}

		if DiffHasChanges(remoteToLocalDiff) {
			err = ApplyFileListDiffWorkspace(apiClient, state.ProjectId, state.WorkspaceInfo.WorkspaceId, changeResp.ChangeId, remoteToLocalDiff)
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
			WorkspaceInfo: &statefile.WorkspaceInfo{
				WorkspaceId: state.WorkspaceInfo.WorkspaceId,
				ChangeId:    changeResp.ChangeId,
			},
		}.Save()
		if err != nil {
			panic(err)
		}
	} else {
		// otherwise, just create a new workspace
		resp, err := apiClient.CreateWorkspace(ctx, &pb.CreateWorkspaceRequest{ProjectId: state.ProjectId, WorkspaceName: os.Args[2]})
		if err != nil {
			log.Panic(err)
		}

		err = statefile.StateFile{
			ProjectId: state.ProjectId,
			WorkspaceInfo: &statefile.WorkspaceInfo{
				WorkspaceId: resp.WorkspaceId,
			},
		}.Save()
		if err != nil {
			panic(err)
		}
		fmt.Println("Switched to new workspace", os.Args[2]+".")
	}
}
