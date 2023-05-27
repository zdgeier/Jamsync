package client

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	serverclient "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"github.com/zdgeier/jamsync/internal/statefile"
	"golang.org/x/oauth2"
)

func Push() {
	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	stateFile, err := statefile.Find()
	if err != nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
		os.Exit(1)
	}

	apiClient, closer, err := server.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	client := serverclient.NewClient(apiClient, stateFile.ProjectId, stateFile.BranchId)

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
		for key, val := range localToRemoteDiff.GetDiffs() {
			if val.Type != pb.FileMetadataDiff_NoOp {
				fmt.Println("Pushed", key)
			}
		}
	} else {
		fmt.Println("No changes to push")
	}
}
