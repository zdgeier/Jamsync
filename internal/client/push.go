package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/zdgeier/jamsync/gen/pb"
	serverclient "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func Push() {
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

	config, _ := findJamsyncConfig()
	if config == nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
		return
	}

	apiClient, closer, err := server.Connect(&oauth2.Token{
		AccessToken: string(accessToken),
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	client := serverclient.NewClient(apiClient, config.ProjectId, config.BranchId)

	fileMetadata := readLocalFileList()
	localToRemoteDiff, err := client.DiffLocalToRemote(context.Background(), fileMetadata)
	if err != nil {
		log.Panic(err)
	}

	if diffHasChanges(localToRemoteDiff) {
		err = pushFileListDiff(fileMetadata, localToRemoteDiff, client)
		if err != nil {
			log.Panic(err)
		}
		writeJamsyncFile(client.ProjectConfig())
		for key, val := range localToRemoteDiff.GetDiffs() {
			if val.Type != pb.FileMetadataDiff_NoOp {
				fmt.Println("Pushed", key)
			}
		}
	} else {
		fmt.Println("No changes to push")
	}
}
