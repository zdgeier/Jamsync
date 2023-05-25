package api

import (
	"bytes"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
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

func ProjectBrowseHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		var branchId uint64
		if ctx.Param("branchName") == "main" {
			branchId = 0
		} else {
			branchResp, err := tempClient.GetBranch(ctx, &pb.GetBranchRequest{
				ProjectId:  config.ProjectId,
				BranchName: ctx.Param("branchName"),
			})
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
			branchId = branchResp.GetBranchId()
		}

		client := client.NewClient(tempClient, config.GetProjectId(), uint64(branchId))
		resp, err := client.BrowseProject(ctx.Param("path")[1:])
		if err != nil {
			ctx.Error(err)
			return
		}
		ctx.JSON(200, resp)
	}
}

func GetFileHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken := sessions.Default(ctx).Get("access_token").(string)
		tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
			ProjectName: ctx.Param("projectName"),
		})
		if err != nil {
			ctx.Error(err)
			return
		}

		var branchId uint64
		if ctx.Param("branchName") == "main" {
			branchId = 0
		} else {
			branchResp, err := tempClient.GetBranch(ctx, &pb.GetBranchRequest{
				ProjectId:  config.ProjectId,
				BranchName: ctx.Param("branchName"),
			})
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
			branchId = branchResp.GetBranchId()
		}

		client := client.NewClient(tempClient, config.GetProjectId(), uint64(branchId))

		client.DownloadFile(ctx, ctx.Param("path")[1:], bytes.NewReader([]byte{}), ctx.Writer)
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

		config, err := tempClient.GetProjectConfig(ctx, &pb.GetProjectConfigRequest{
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
