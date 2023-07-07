package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jam"
	"github.com/zdgeier/jamhub/internal/jam/authfile"
	"github.com/zdgeier/jamhub/internal/jam/statefile"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

// App struct
type App struct {
	ctx    context.Context
	client pb.JamHubClient
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	authfile.Logout()
	a.ctx = ctx
	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	apiClient, _, err := jamhubgrpc.Connect(&oauth2.Token{
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

func (a *App) WorkOn(workspaceName string) string {
	state, err := statefile.Find()
	if err != nil {
		panic("Could not find a `.jamhub` file. Run `jam init` to initialize the project.")
	}

	if state.CommitInfo == nil || state.WorkspaceInfo != nil {
		if workspaceName == "main" || workspaceName == "mainline" {
			fileMetadata := jam.ReadLocalFileList()
			localToRemoteDiff, err := jam.DiffLocalToRemoteWorkspace(a.client, state.ProjectId, state.WorkspaceInfo.WorkspaceId, state.WorkspaceInfo.ChangeId, fileMetadata)
			if err != nil {
				log.Panic(err)
			}
			if jam.DiffHasChanges(localToRemoteDiff) {
				return "Some changes locally have not been pushed. Run `jam push` to push your local changes."
			}

			commitResp, err := a.client.GetProjectCurrentCommit(context.Background(), &pb.GetProjectCurrentCommitRequest{
				ProjectId: state.ProjectId,
			})
			if err != nil {
				log.Panic(err)
			}

			diffRemoteToLocalResp, err := jam.DiffRemoteToLocalCommit(a.client, state.ProjectId, commitResp.CommitId, &pb.FileMetadata{})
			if err != nil {
				log.Panic(err)
			}

			err = jam.ApplyFileListDiffCommit(a.client, state.ProjectId, commitResp.CommitId, diffRemoteToLocalResp)
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
		} else {
			return "Must be on mainline to workon."
		}
	}

	if workspaceName == "main" || workspaceName == "mainline" {
		fmt.Println("`main` and `mainline` are workspace names reserved for commits. Please choose another workspace name.")
		os.Exit(1)
	}

	resp, err := a.client.ListWorkspaces(a.ctx, &pb.ListWorkspacesRequest{ProjectId: state.ProjectId})
	if err != nil {
		panic(err)
	}

	if workspaceId, ok := resp.GetWorkspaces()[workspaceName]; ok {
		if workspaceId == state.WorkspaceInfo.WorkspaceId {
			return fmt.Sprintf("%s %s", "Already on", workspaceName)
		}

		changeResp, err := a.client.GetWorkspaceCurrentChange(context.TODO(), &pb.GetWorkspaceCurrentChangeRequest{ProjectId: state.ProjectId, WorkspaceId: workspaceId})
		if err != nil {
			panic(err)
		}

		// if workspace already exists, do a pull
		fileMetadata := jam.ReadLocalFileList()
		remoteToLocalDiff, err := jam.DiffRemoteToLocalWorkspace(a.client, state.ProjectId, state.WorkspaceInfo.WorkspaceId, changeResp.ChangeId, fileMetadata)
		if err != nil {
			log.Panic(err)
		}

		if jam.DiffHasChanges(remoteToLocalDiff) {
			err = jam.ApplyFileListDiffWorkspace(a.client, state.ProjectId, state.WorkspaceInfo.WorkspaceId, changeResp.ChangeId, remoteToLocalDiff)
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
		return ""
	}

	// otherwise, just create a new workspace
	createResp, err := a.client.CreateWorkspace(a.ctx, &pb.CreateWorkspaceRequest{ProjectId: state.ProjectId, WorkspaceName: workspaceName})
	if err != nil {
		log.Panic(err)
	}

	err = statefile.StateFile{
		ProjectId: state.ProjectId,
		WorkspaceInfo: &statefile.WorkspaceInfo{
			WorkspaceId: createResp.WorkspaceId,
		},
	}.Save()
	if err != nil {
		panic(err)
	}
	return fmt.Sprint("Switched to new workspace ", workspaceName, ".")
}

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
	jam.InitNewProject(a.client, projectName)
}

func (a *App) InitExistingProject(path string, projectName string) {
	err := os.Chdir(path)
	if err != nil {
		panic(err)
	}
	jam.InitExistingProject(a.client, projectName)
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
		panic("no .jamhub!")
	}
	nameResp, err := a.client.GetProjectName(context.Background(), &pb.GetProjectNameRequest{
		ProjectId: state.ProjectId,
	})
	if err != nil {
		panic(err)
	}
	res = append(res, fmt.Sprintf("Project: %s\n", nameResp.ProjectName))

	if state.WorkspaceInfo != nil {
		workspaceNameResp, err := a.client.GetWorkspaceName(context.Background(), &pb.GetWorkspaceNameRequest{
			ProjectId:   state.ProjectId,
			WorkspaceId: state.WorkspaceInfo.WorkspaceId,
		})
		if err != nil {
			panic(err)
		}
		res = append(res, fmt.Sprintf(
			"Workspace:  %s\n",
			workspaceNameResp.GetWorkspaceName(),
		))

		res = append(res, fmt.Sprintf(
			"Change:  %d\n",
			state.WorkspaceInfo.ChangeId,
		))

		changeResp, err := a.client.GetWorkspaceCurrentChange(context.Background(), &pb.GetWorkspaceCurrentChangeRequest{ProjectId: state.ProjectId, WorkspaceId: state.WorkspaceInfo.WorkspaceId})
		if err != nil {
			panic(err)
		}

		if changeResp.ChangeId == state.WorkspaceInfo.ChangeId {
			fileMetadata := jam.ReadLocalFileList()
			localToRemoteDiff, err := jam.DiffLocalToRemoteWorkspace(a.client, state.ProjectId, state.WorkspaceInfo.WorkspaceId, state.WorkspaceInfo.ChangeId, fileMetadata)
			if err != nil {
				log.Panic(err)
			}

			if jam.DiffHasChanges(localToRemoteDiff) {
				res = append(res, "\nModified files:")
				for path, diff := range localToRemoteDiff.Diffs {
					if diff.Type != pb.FileMetadataDiff_NoOp {
						res = append(res, "\n  "+path)
					}
				}
			} else {
				res = append(res, "\nNo local or remote changes.")
			}
		} else if changeResp.ChangeId > state.WorkspaceInfo.ChangeId {
			fileMetadata := jam.ReadLocalFileList()
			remoteToLocalDiff, err := jam.DiffRemoteToLocalWorkspace(a.client, state.ProjectId, state.WorkspaceInfo.WorkspaceId, state.WorkspaceInfo.ChangeId, fileMetadata)
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
