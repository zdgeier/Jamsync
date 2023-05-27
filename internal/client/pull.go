package client

import (
	"context"
	"fmt"
	"log"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	serverclient "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"github.com/zdgeier/jamsync/internal/statefile"
	"golang.org/x/oauth2"
)

func Pull() {
	state, err := statefile.Find()
	if err != nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
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

	client := serverclient.NewClient(apiClient, state.ProjectId, state.BranchId)

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
}
