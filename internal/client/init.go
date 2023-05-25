package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/zdgeier/jamsync/gen/pb"
	serverclient "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
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

	client := serverclient.NewClient(apiClient, resp.ProjectId, 0)

	fileMetadata := readLocalFileList()
	fileMetadataDiff, err := client.DiffLocalToRemote(context.Background(), fileMetadata)
	if err != nil {
		panic(err)
	}

	err = pushFileListDiff(fileMetadata, fileMetadataDiff, client)
	if err != nil {
		panic(err)
	}

	fmt.Println("Merging...")
	_, err = apiClient.MergeBranch(context.Background(), &pb.MergeBranchRequest{
		ProjectId: resp.ProjectId,
		BranchId:  0,
	})
	if err != nil {
		panic(err)
	}

	_, err = apiClient.DeleteBranch(context.Background(), &pb.DeleteBranchRequest{
		ProjectId: resp.ProjectId,
		BranchId:  0,
	})
	if err != nil {
		log.Panic(err)
	}

	err = writeJamsyncFile(client.ProjectConfig())
	if err != nil {
		panic(err)
	}
}

func initExistingProject(apiClient pb.JamsyncAPIClient) {
	fmt.Print("Name of project to download: ")
	var projectName string
	fmt.Scan(&projectName)

	resp, err := apiClient.GetProjectConfig(context.Background(), &pb.GetProjectConfigRequest{
		ProjectName: projectName,
	})
	if err != nil {
		log.Panic(err)
	}

	client := serverclient.NewClient(apiClient, resp.ProjectId, 0)

	diffRemoteToLocalResp, err := client.DiffRemoteToLocal(context.Background(), &pb.FileMetadata{})
	if err != nil {
		log.Panic(err)
	}

	err = applyFileListDiff(diffRemoteToLocalResp, client)
	if err != nil {
		log.Panic(err)
	}

	err = writeJamsyncFile(client.ProjectConfig())
	if err != nil {
		log.Panic(err)
	}
}

func InitConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Panic(err)
	}
	accessToken, err := os.ReadFile(authPath(home))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("Run `jam login` to login to Jamsync first (" + home + "/.jamsyncauth does not exist).")
		os.Exit(1)
	} else if err != nil {
		panic(err)
	}

	config, path := findJamsyncConfig()
	if config != nil {
		fmt.Println("There's already a config at " + path + ".")
		os.Exit(1)
	}

	apiClient, closer, err := server.Connect(&oauth2.Token{
		AccessToken: string(accessToken),
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	_, err = apiClient.Ping(context.Background(), &pb.PingRequest{})
	if err != nil {
		fmt.Println(err)
		loginAuth()
	}

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
