package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/pkg/browser"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/jamenv"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func Open() {
	config, _ := findJamsyncConfig()
	if config == nil {
		fmt.Println("Could not find a `.jamsync` file. Run `jam init` to initialize the project.")
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		log.Panic(err)
	}
	accessToken, err := os.ReadFile(authPath(home))
	if errors.Is(err, os.ErrNotExist) {
		loginAuth()
		return
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

	pingRes, err := apiClient.Ping(context.Background(), &pb.PingRequest{})
	if err != nil {
		loginAuth()
	}
	configRes, err := apiClient.GetProjectConfig(context.Background(), &pb.GetProjectConfigRequest{
		ProjectId: config.ProjectId,
	})
	if err != nil {
		panic(err)
	}

	url := "https://jamsync.dev/"
	if jamenv.Env() == jamenv.Local {
		url = "http://localhost:8081/"
	}

	err = browser.OpenURL(url + pingRes.GetUsername() + "/" + configRes.ProjectName + "/files/main")
	if err != nil {
		panic(err)
	}
}
