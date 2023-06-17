package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/authfile"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

// App struct
type App struct {
	ctx    context.Context
	client pb.JamsyncAPIClient
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	authFile, err := authfile.Authorize()
	if err != nil {
		panic(err)
	}

	apiClient, _, err := server.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
	})
	if err != nil {
		log.Panic(err)
	}
	a.client = apiClient
}

func (a *App) ListProjects() []string {
	resp, err := a.client.ListUserProjects(a.ctx, &pb.ListUserProjectsRequest{})
	if err != nil {
		log.Panic(err)
	}

	var projects []string
	for _, proj := range resp.GetProjects() {
		projects = append(projects, proj.Name)
	}
	return projects
}

func (a *App) SelectDirectory() {
	_, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
	if err != nil {
		panic(err)
	}
}
