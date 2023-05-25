package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zdgeier/jamsync/gen/pb"
	serverclient "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func Checkout() {
	if len(os.Args) != 3 {
		fmt.Println("jam checkout <branch name>")
		return
	}
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

	apiClient, closer, err := server.Connect(&oauth2.Token{
		AccessToken: string(accessToken),
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err != nil {
		log.Panic(err)
	}

	config, _ := findJamsyncConfig()
	if config == nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
		return
	}

	client := serverclient.NewClient(apiClient, config.ProjectId, config.BranchId)

	resp, err := apiClient.ListBranches(ctx, &pb.ListBranchesRequest{ProjectId: client.ProjectConfig().ProjectId})
	if err != nil {
		log.Panic(err)
	}

	if branchId, ok := resp.GetBranches()[os.Args[2]]; ok {
		if branchId == config.BranchId {
			fmt.Println("Already on", os.Args[2])
			return
		}
		// if branch already exists, do a pull
		fileMetadata := readLocalFileList()
		remoteToLocalDiff, err := client.DiffRemoteToLocal(context.Background(), fileMetadata)
		if err != nil {
			log.Panic(err)
		}

		if diffHasChanges(remoteToLocalDiff) {
			err = applyFileListDiff(remoteToLocalDiff, client)
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
		client := serverclient.NewClient(apiClient, config.ProjectId, branchId)
		writeJamsyncFile(client.ProjectConfig())
	} else {
		// otherwise, just create a new branch
		resp, err := apiClient.CreateBranch(ctx, &pb.CreateBranchRequest{ProjectId: client.ProjectConfig().ProjectId, BranchName: os.Args[2]})
		if err != nil {
			log.Panic(err)
		}
		client := serverclient.NewClient(apiClient, config.ProjectId, resp.GetBranchId())
		writeJamsyncFile(client.ProjectConfig())
		fmt.Println("Switched to new branch", os.Args[2], "with id", resp.GetBranchId())
	}
}
