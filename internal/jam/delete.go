package jam

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jam/authfile"
	"github.com/zdgeier/jamhub/internal/jam/statefile"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

func Delete() {
	state, err := statefile.Find()
	if err != nil {
		fmt.Println("Could not find a `.jam` file. Run `jam init` to initialize the project.")
		os.Exit(0)
	}

	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	apiClient, closer, err := jamhubgrpc.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
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

	if err == nil {
		resp, err := apiClient.DeleteProject(ctx, &pb.DeleteProjectRequest{
			ProjectId: state.ProjectId,
		})
		if err != nil {
			log.Panic(err)
		}
		err = os.Remove(".jam")
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
