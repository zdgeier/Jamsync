package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	"github.com/zdgeier/jamsync/internal/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"github.com/zdgeier/jamsync/internal/statefile"
	"golang.org/x/oauth2"
)

// App struct
type App struct {
	ctx    context.Context
	client pb.JamsyncAPIClient
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	apiClient, _, err := server.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
	})
	if err != nil {
		log.Panic(err)
	}
	a.client = apiClient
}

// func (a *App) ListProjects() []string {
// 	resp, err := a.client.ListUserProjects(a.ctx, &pb.ListUserProjectsRequest{})
// 	if err != nil {
// 		log.Panic(err)
// 	}
//
// 	var projects []string
// 	for _, proj := range resp.GetProjects() {
// 		projects = append(projects, proj.Name)
// 	}
// 	return projects
// }

func (a *App) ProjectExists(projectName string) bool {
	resp, err := a.client.ListUserProjects(a.ctx, &pb.ListUserProjectsRequest{})
	if err != nil {
		log.Panic(err)
	}

	for _, proj := range resp.GetProjects() {
		if proj.Name == projectName {
			return true
		}
	}
	return false
}

func (a *App) InitNewProject(path string, projectName string) {
	err := os.Chdir(path)
	if err != nil {
		panic(err)
	}
	client.InitNewProject(a.client, projectName)
}

func (a *App) InitExistingProject(path string, projectName string) {
	err := os.Chdir(path)
	if err != nil {
		panic(err)
	}
	client.InitExistingProject(a.client, projectName)
}

func (a *App) ChangeDirectory(path string) {
	err := os.Chdir(path)
	if err != nil {
		panic(err)
	}
}

func (a *App) StateFileExists(path string) bool {
	_, err := statefile.Find()
	return err == nil
}

// returns true if existing project, false
func (a *App) SelectDirectory() string {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
	if err != nil {
		panic(err)
	}
	return path
}

func (a *App) GetInfo() []string {
	res := make([]string, 0)

	state, err := statefile.Find()
	if err != nil {
		panic("no .jamsync!")
	}
	nameResp, err := a.client.GetProjectName(context.Background(), &pb.GetProjectNameRequest{
		ProjectId: state.ProjectId,
	})
	if err != nil {
		panic(err)
	}
	res = append(res, fmt.Sprintf("Project: %s\n", nameResp.ProjectName))

	if state.BranchInfo != nil {
		branchNameResp, err := a.client.GetBranchName(context.Background(), &pb.GetBranchNameRequest{
			ProjectId: state.ProjectId,
			BranchId:  state.BranchInfo.BranchId,
		})
		if err != nil {
			panic(err)
		}
		res = append(res, fmt.Sprintf(
			"Branch:  %s\n",
			branchNameResp.GetBranchName(),
		))

		res = append(res, fmt.Sprintf(
			"Change:  %d\n",
			state.BranchInfo.ChangeId,
		))

		changeResp, err := a.client.GetBranchCurrentChange(context.Background(), &pb.GetBranchCurrentChangeRequest{ProjectId: state.ProjectId, BranchId: state.BranchInfo.BranchId})
		if err != nil {
			panic(err)
		}

		if changeResp.ChangeId == state.BranchInfo.ChangeId {
			fileMetadata := client.ReadLocalFileList()
			localToRemoteDiff, err := client.DiffLocalToRemoteBranch(a.client, state.ProjectId, state.BranchInfo.BranchId, state.BranchInfo.ChangeId, fileMetadata)
			if err != nil {
				log.Panic(err)
			}

			if client.DiffHasChanges(localToRemoteDiff) {
				res = append(res, "\nModified files:")
				for path, diff := range localToRemoteDiff.Diffs {
					if diff.Type != pb.FileMetadataDiff_NoOp {
						res = append(res, "\n  "+path)
					}
				}
			} else {
				res = append(res, "\nNo local or remote changes.")
			}
		} else if changeResp.ChangeId > state.BranchInfo.ChangeId {
			fileMetadata := client.ReadLocalFileList()
			remoteToLocalDiff, err := client.DiffRemoteToLocalBranch(a.client, state.ProjectId, state.BranchInfo.BranchId, state.BranchInfo.ChangeId, fileMetadata)
			if err != nil {
				log.Panic(err)
			}

			for path := range remoteToLocalDiff.GetDiffs() {
				res = append(res, path+" changed")
			}
		} else {
			fmt.Println("invalid state: local change id greater than remote change id")
		}
	} else {
		res = append(res, fmt.Sprintf("Commit:  %d\n", state.CommitInfo.CommitId))
	}

	return res
}
