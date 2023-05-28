package client

import (
	"context"
	"fmt"

	"github.com/pkg/browser"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	"github.com/zdgeier/jamsync/internal/jamenv"
	"github.com/zdgeier/jamsync/internal/server/server"
	"github.com/zdgeier/jamsync/internal/statefile"
	"golang.org/x/oauth2"
)

func Open() {
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
		panic(err)
	}
	defer closer()

	nameResp, err := apiClient.GetProjectName(context.Background(), &pb.GetProjectNameRequest{
		ProjectId: state.ProjectId,
	})
	if err != nil {
		panic(err)
	}

	url := "https://jamsync.dev/"
	username := authFile.Username
	if jamenv.Env() == jamenv.Local {
		url = "http://localhost:8081/"
		username = "test@jamsync.dev"
	}

	err = browser.OpenURL(url + username + "/" + nameResp.ProjectName + "/committedfiles/")
	if err != nil {
		panic(err)
	}
}
