package client

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	"github.com/zdgeier/jamsync/internal/server/server"
	"github.com/zdgeier/jamsync/internal/statefile"
	"golang.org/x/oauth2"
)

func initNewProject(apiClient pb.JamsyncAPIClient) {
	fmt.Print("Project Name: ")
	var projectName string
	fmt.Scan(&projectName)

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
	fmt.Println("Initializing a project at " + currentPath)

	branchResp, err := apiClient.CreateBranch(context.TODO(), &pb.CreateBranchRequest{ProjectId: resp.ProjectId, BranchName: "init"})
	if err != nil {
		log.Panic(err)
	}

	fileMetadata := readLocalFileList()
	fileMetadataDiff, err := diffLocalToRemoteCommit(apiClient, resp.GetProjectId(), branchResp.GetBranchId(), fileMetadata)
	if err != nil {
		panic(err)
	}

	_, err = pushFileListDiffBranch(apiClient, resp.ProjectId, branchResp.BranchId, 0, fileMetadata, fileMetadataDiff)
	if err != nil {
		panic(err)
	}

	fmt.Println("Merging...")
	mergeResp, err := apiClient.MergeBranch(context.Background(), &pb.MergeBranchRequest{
		ProjectId: resp.ProjectId,
		BranchId:  branchResp.BranchId,
	})
	if err != nil {
		panic(err)
	}

	_, err = apiClient.DeleteBranch(context.Background(), &pb.DeleteBranchRequest{
		ProjectId: resp.ProjectId,
		BranchId:  branchResp.BranchId,
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
}

func initExistingProject(apiClient pb.JamsyncAPIClient) {
	fmt.Print("Name of project to download: ")
	var projectName string
	fmt.Scan(&projectName)

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

	diffRemoteToLocalResp, err := diffRemoteToLocalCommit(apiClient, resp.ProjectId, commitResp.CommitId, &pb.FileMetadata{})
	if err != nil {
		log.Panic(err)
	}

	err = applyFileListDiffCommit(apiClient, resp.GetProjectId(), commitResp.CommitId, diffRemoteToLocalResp)
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
		fmt.Println("There's already a project initialized file here. Remove the `.jamsync` file to reinitialize.")
		os.Exit(1)
	}

	authFile, err := authfile.Authorize()
	if err != nil {
		fmt.Println("`~/.jamsyncauth` file could not be found. Run `jam login` to create this file.")
		os.Exit(1)
	}

	apiClient, closer, err := server.Connect(&oauth2.Token{
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
			initNewProject(apiClient)
			break
		} else if strings.ToLower(flag) == "n" {
			initExistingProject(apiClient)
			break
		}
	}
}
