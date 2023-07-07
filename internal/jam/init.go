package jam

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jam/authfile"
	"github.com/zdgeier/jamhub/internal/jam/statefile"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

func InitNewProject(apiClient pb.JamHubClient, projectName string) {
	resp, err := apiClient.AddProject(context.Background(), &pb.AddProjectRequest{
		ProjectName: projectName,
	})
	if err != nil {
		panic(err)
	}
	currentPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println("Initializing a project at " + currentPath + ". Uploading files...")

	workspaceResp, err := apiClient.CreateWorkspace(context.TODO(), &pb.CreateWorkspaceRequest{ProjectId: resp.ProjectId, WorkspaceName: "init"})
	if err != nil {
		log.Panic(err)
	}

	fileMetadata := ReadLocalFileList()
	fileMetadataDiff, err := diffLocalToRemoteCommit(apiClient, resp.GetProjectId(), workspaceResp.GetWorkspaceId(), fileMetadata)
	if err != nil {
		panic(err)
	}

	err = pushFileListDiffWorkspace(apiClient, resp.ProjectId, workspaceResp.WorkspaceId, 0, fileMetadata, fileMetadataDiff)
	if err != nil {
		panic(err)
	}

	fmt.Println("Merging...")
	mergeResp, err := apiClient.MergeWorkspace(context.Background(), &pb.MergeWorkspaceRequest{
		ProjectId:   resp.ProjectId,
		WorkspaceId: workspaceResp.WorkspaceId,
	})
	if err != nil {
		panic(err)
	}

	_, err = apiClient.DeleteWorkspace(context.Background(), &pb.DeleteWorkspaceRequest{
		ProjectId:   resp.ProjectId,
		WorkspaceId: workspaceResp.WorkspaceId,
	})
	if err != nil {
		log.Panic(err)
	}

	err = statefile.StateFile{
		ProjectId: resp.ProjectId,
		CommitInfo: &statefile.CommitInfo{
			CommitId: mergeResp.CommitId,
		},
	}.Save()
	if err != nil {
		panic(err)
	}
	fmt.Println("Done! Run `jam workon <workspace name>` to start making changes.")
}

func InitExistingProject(apiClient pb.JamHubClient, projectName string) {
	resp, err := apiClient.GetProjectId(context.Background(), &pb.GetProjectIdRequest{
		ProjectName: projectName,
	})
	if err != nil {
		log.Panic(err)
	}

	commitResp, err := apiClient.GetProjectCurrentCommit(context.Background(), &pb.GetProjectCurrentCommitRequest{
		ProjectId: resp.GetProjectId(),
	})
	if err != nil {
		log.Panic(err)
	}

	diffRemoteToLocalResp, err := DiffRemoteToLocalCommit(apiClient, resp.ProjectId, commitResp.CommitId, &pb.FileMetadata{})
	if err != nil {
		log.Panic(err)
	}

	err = ApplyFileListDiffCommit(apiClient, resp.GetProjectId(), commitResp.CommitId, diffRemoteToLocalResp)
	if err != nil {
		log.Panic(err)
	}

	err = statefile.StateFile{
		ProjectId: resp.ProjectId,
		CommitInfo: &statefile.CommitInfo{
			CommitId: commitResp.CommitId,
		},
	}.Save()
	if err != nil {
		panic(err)
	}
}

func InitConfig() {
	_, err := statefile.Find()
	if err == nil {
		fmt.Println("There's already a project initialized file here. Remove the `.jamhub` file to reinitialize.")
		os.Exit(1)
	}

	authFile, err := authfile.Authorize()
	if err != nil {
		fmt.Println("`~/.jamhubauth` file could not be found. Run `jam login` to create this file.")
		os.Exit(1)
	}

	apiClient, closer, err := jamhubgrpc.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	for {
		fmt.Print("Create a new project (y/n)? ")
		var flag string
		fmt.Scanln(&flag)
		if strings.ToLower(flag) == "y" {
			fmt.Print("Project Name: ")
			var projectName string
			fmt.Scan(&projectName)
			InitNewProject(apiClient, projectName)
			break
		} else if strings.ToLower(flag) == "n" {
			fmt.Print("Name of project to download: ")
			var projectName string
			fmt.Scan(&projectName)
			InitExistingProject(apiClient, projectName)
			break
		}
	}
}
