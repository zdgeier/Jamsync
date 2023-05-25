package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func Delete() {
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
	if config != nil {
		resp, err := apiClient.DeleteProject(ctx, &pb.DeleteProjectRequest{
			ProjectId: config.ProjectId,
		})
		if err != nil {
			log.Panic(err)
		}
		err = os.Remove(".jamsync")
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("Deleted " + resp.ProjectName)
	} else {
		resp, err := apiClient.DeleteProject(ctx, &pb.DeleteProjectRequest{
			ProjectName: os.Args[1],
		})
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("Deleted " + resp.ProjectName)
	}
}
