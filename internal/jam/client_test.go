package jam

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jamenv"
	"github.com/zdgeier/jamhub/internal/jamhub/file"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

var serverRunning = false

func setup() (pb.JamHubClient, func(), error) {
	if !serverRunning {
		if jamenv.Env() == jamenv.Local {
			err := os.RemoveAll("jamhubdata/")
			if err != nil {
				log.Panic(err)
			}
			err = os.RemoveAll("jamhub.db")
			if err != nil {
				log.Panic(err)
			}
		}
		_, err := jamhubgrpc.New()
		if err != nil && !strings.Contains(err.Error(), "bind: address already in use") {
			return nil, nil, err
		}
		serverRunning = true
	}

	return jamhubgrpc.Connect(&oauth2.Token{AccessToken: ""})
}

func TestClient_UploadDownloadWorkspaceFile(t *testing.T) {
	ctx := context.Background()

	apiClient, closer, err := setup()
	require.NoError(t, err)
	defer closer()

	projectName := "UploadDownloadWorkspaceFile"

	addProjectResp, err := apiClient.AddProject(context.Background(), &pb.AddProjectRequest{
		ProjectName: projectName,
	})
	require.NoError(t, err)

	resp, err := apiClient.CreateWorkspace(ctx, &pb.CreateWorkspaceRequest{ProjectId: addProjectResp.ProjectId, WorkspaceName: os.Args[2]})
	if err != nil {
		log.Panic(err)
	}

	fileOperations := []struct {
		name     string
		filePath string
		data     []byte
	}{
		{
			name:     "test1",
			filePath: "test",
			data:     []byte("this is a test!"),
		},
		{
			name:     "test2",
			filePath: "test2",
			data:     []byte("this is a test!"),
		},
		{
			name:     "new path",
			filePath: "this/is/a/path.txt",
			data:     []byte("this is a test!this is a test!this is a test!this is a test!this is a test!this is a test!"),
		},
		{
			name:     "reused path",
			filePath: "this/is/a/path.txt",
			data:     []byte("xthis is a test!this is a test!this is a test!this is a test!this is a test!this is a test!"),
		},
		{
			name:     "reused path",
			filePath: "this/is/a/path.txt",
			data:     []byte("this is a test!this is a test!this is a test!this is a test!this is a test!this is a test!x"),
		},
		{
			name:     "reused path",
			filePath: "this/is/a/path.txt",
			data:     []byte("this is a test!this is a test!this is a test!this is a test!this is a test!this is a test!!this is a test!"),
		},
		{
			name:     "reused path",
			filePath: "this/is/a/path.txt",
			data:     []byte("this is a test!this is a test!this is a test!this is a test!this is a test!this is a test!!this is a test!"),
		},
	}

	var changeId uint64
	for _, fileOperation := range fileOperations {
		t.Run(fileOperation.name, func(t *testing.T) {
			err = uploadWorkspaceFile(apiClient, addProjectResp.ProjectId, resp.WorkspaceId, changeId, fileOperation.filePath, bytes.NewReader(fileOperation.data))
			require.NoError(t, err)

			result := new(bytes.Buffer)
			err = file.DownloadWorkspaceFile(apiClient, addProjectResp.ProjectId, resp.WorkspaceId, changeId, fileOperation.filePath, bytes.NewReader(fileOperation.data), result)

			require.NoError(t, err)
			require.Equal(t, fileOperation.data, result.Bytes())
			changeId += 1
		})
	}
}

func TestClient_UploadDownloadMergedFile(t *testing.T) {
	ctx := context.Background()

	apiClient, closer, err := setup()
	require.NoError(t, err)
	defer closer()

	projectName := "UploadDownloadMergedFile"

	addProjectResp, err := apiClient.AddProject(context.Background(), &pb.AddProjectRequest{
		ProjectName: projectName,
	})
	require.NoError(t, err)

	fileOperations := []struct {
		name          string
		filePath      string
		data          []byte
		workspaceName string
		changeId      uint64
	}{
		{
			name:          "test1",
			filePath:      "test",
			data:          []byte("this is a test!"),
			workspaceName: "test1",
			changeId:      0,
		},
		{
			name:          "test2",
			filePath:      "test2",
			workspaceName: "test2",
			data:          []byte("this is a test!"),
			changeId:      0,
		},
		{
			name:          "new path",
			filePath:      "this/is/a/path.txt",
			workspaceName: "test3",
			data:          []byte("this is a test!this is a test!this is a test!this is a test!this is a test!this is a test!"),
			changeId:      0,
		},
		{
			name:          "reused path1",
			filePath:      "this/is/a/path.txt",
			workspaceName: "test4",
			data:          []byte("xthis is a test!this is a test!this is a test!this is a test!this is a test!this is a test!"),
			changeId:      0,
		},
		{
			name:          "reused path2",
			filePath:      "this/is/a/path.txt",
			workspaceName: "test5",
			data:          []byte("this is a test!this is a test!this is a test!this is a test!this is a test!this is a test!x"),
			changeId:      0,
		},
		{
			name:          "reused path3",
			filePath:      "this/is/a/path.txt",
			workspaceName: "test6",
			data:          []byte("this is a test!this is a test!this is a test!this is a test!this is a test!this is a test!!this is a test!"),
			changeId:      0,
		},
		{
			name:          "reused path4",
			filePath:      "this/is/a/path.txt",
			workspaceName: "test7",
			data:          []byte("this is a test!this is a test!this is a test!this is a test!this is a test!this is a test!!this is a test!"),
			changeId:      0,
		},
	}

	for _, fileOperation := range fileOperations {
		t.Run(fileOperation.name, func(t *testing.T) {
			resp, err := apiClient.CreateWorkspace(ctx, &pb.CreateWorkspaceRequest{ProjectId: addProjectResp.ProjectId, WorkspaceName: fileOperation.workspaceName})
			if err != nil {
				log.Panic(err)
			}

			err = uploadWorkspaceFile(apiClient, addProjectResp.ProjectId, resp.WorkspaceId, fileOperation.changeId, fileOperation.filePath, bytes.NewReader(fileOperation.data))
			require.NoError(t, err)

			result := new(bytes.Buffer)
			err = file.DownloadWorkspaceFile(apiClient, addProjectResp.ProjectId, resp.WorkspaceId, fileOperation.changeId, fileOperation.filePath, bytes.NewReader(fileOperation.data), result)

			require.NoError(t, err)
			require.Equal(t, fileOperation.data, result.Bytes())

			mergeResp, err := apiClient.MergeWorkspace(ctx, &pb.MergeWorkspaceRequest{ProjectId: addProjectResp.ProjectId, WorkspaceId: resp.WorkspaceId})
			if err != nil {
				log.Panic(err)
			}

			mergeResult := new(bytes.Buffer)
			err = file.DownloadCommittedFile(apiClient, addProjectResp.ProjectId, mergeResp.CommitId, fileOperation.filePath, bytes.NewReader(fileOperation.data), mergeResult)

			require.NoError(t, err)
			require.Equal(t, fileOperation.data, mergeResult.Bytes())
		})
	}
}
