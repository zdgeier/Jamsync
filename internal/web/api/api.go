package api

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/file"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
)

func UserProjectsHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		resp, err := tempClient.ListUserProjects(ctx, &pb.ListUserProjectsRequest{})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.JSON(200, resp)
	}
}

func GetProjectCurrentCommitHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()
		id, err := tempClient.GetProjectId(ctx, &pb.GetProjectIdRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		resp, err := tempClient.GetProjectCurrentCommit(ctx, &pb.GetProjectCurrentCommitRequest{ProjectId: id.GetProjectId()})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		ctx.JSON(200, resp)
	}
}

func GetBranchInfoHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		id, err := tempClient.GetProjectId(ctx, &pb.GetProjectIdRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		branchIdResponse, err := tempClient.GetBranchId(ctx, &pb.GetBranchIdRequest{ProjectId: id.GetProjectId(), BranchName: ctx.Param("branchName")})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		resp, err := tempClient.GetBranchCurrentChange(ctx, &pb.GetBranchCurrentChangeRequest{ProjectId: id.GetProjectId(), BranchId: branchIdResponse.BranchId})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		type branchInfo struct {
			BranchId uint64 `json:"branch_id"`
			ChangeId uint64 `json:"change_id"`
		}

		ctx.JSON(200, &branchInfo{
			BranchId: branchIdResponse.BranchId,
			ChangeId: resp.ChangeId,
		})
	}
}

func ProjectBrowseCommitHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		id, err := tempClient.GetProjectId(ctx, &pb.GetProjectIdRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		commitId, err := strconv.Atoi(ctx.Param("commitId"))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		metadataResult := new(bytes.Buffer)
		err = file.DownloadCommittedFile(tempClient, id.GetProjectId(), uint64(commitId), ".jamsyncfilelist", bytes.NewReader([]byte{}), metadataResult)
		if err != nil {
			ctx.Error(err)
			return
		}

		fileMetadata := &pb.FileMetadata{}
		err = proto.Unmarshal(metadataResult.Bytes(), fileMetadata)
		if err != nil {
			ctx.Error(err)
			return
		}

		directoryNames := make([]string, 0, len(fileMetadata.GetFiles()))
		fileNames := make([]string, 0, len(fileMetadata.GetFiles()))
		requestPath := filepath.Clean(ctx.Param("path")[1:])
		for path, file := range fileMetadata.GetFiles() {
			pathDir := filepath.Dir(path)
			if (path == "" && pathDir == ".") || pathDir == requestPath {
				if file.GetDir() {
					directoryNames = append(directoryNames, filepath.Base(path))
				} else {
					fileNames = append(fileNames, filepath.Base(path))
				}
			}
		}

		ctx.JSON(200, &pb.BrowseProjectResponse{
			Directories: directoryNames,
			Files:       fileNames,
		})
	}
}

func ProjectBrowseBranchHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		id, err := tempClient.GetProjectId(ctx, &pb.GetProjectIdRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		branchId, err := strconv.Atoi(ctx.Param("branchId"))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		changeId, err := strconv.Atoi(ctx.Param("changeId"))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		metadataResult := new(bytes.Buffer)
		err = file.DownloadBranchFile(tempClient, id.GetProjectId(), uint64(branchId), uint64(changeId), ".jamsyncfilelist", bytes.NewReader([]byte{}), metadataResult)
		if err != nil {
			ctx.Error(err)
			return
		}
		fileMetadata := &pb.FileMetadata{}
		err = proto.Unmarshal(metadataResult.Bytes(), fileMetadata)
		if err != nil {
			ctx.Error(err)
			return
		}

		directoryNames := make([]string, 0, len(fileMetadata.GetFiles()))
		fileNames := make([]string, 0, len(fileMetadata.GetFiles()))
		requestPath := filepath.Clean(ctx.Param("path")[1:])
		for path, file := range fileMetadata.GetFiles() {
			pathDir := filepath.Dir(path)
			if (path == "" && pathDir == ".") || pathDir == requestPath {
				if file.GetDir() {
					directoryNames = append(directoryNames, filepath.Base(path))
				} else {
					fileNames = append(fileNames, filepath.Base(path))
				}
			}
		}

		ctx.JSON(200, &pb.BrowseProjectResponse{
			Directories: directoryNames,
			Files:       fileNames,
		})
	}
}

func GetFileBranchHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectId(ctx, &pb.GetProjectIdRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.Error(err)
			return
		}

		branchId, err := strconv.Atoi(ctx.Param("branchId"))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		changeId, err := strconv.Atoi(ctx.Param("changeId"))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		err = file.DownloadBranchFile(tempClient, config.ProjectId, uint64(branchId), uint64(changeId), ctx.Param("path")[1:], bytes.NewReader([]byte{}), ctx.Writer)
		if err != nil {
			ctx.Error(err)
			return
		}
	}
}

func GetFileCommitHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectId(ctx, &pb.GetProjectIdRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.Error(err)
			return
		}

		commitId, err := strconv.Atoi(ctx.Param("commitId"))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		err = file.DownloadCommittedFile(tempClient, config.ProjectId, uint64(commitId), ctx.Param("path")[1:], bytes.NewReader([]byte{}), ctx.Writer)
		if err != nil {
			ctx.Error(err)
			return
		}
	}
}

func GetBranchesHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectId(ctx, &pb.GetProjectIdRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		resp, err := tempClient.ListBranches(ctx, &pb.ListBranchesRequest{
			ProjectId: config.ProjectId,
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		ctx.JSON(200, resp)
	}
}
