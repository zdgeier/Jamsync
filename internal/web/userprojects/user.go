package userprojects

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/server"
	"golang.org/x/oauth2"
)

func Handler(ctx *gin.Context) {
	session := sessions.Default(ctx)

	type templateParams struct {
		Email interface{}
	}
	ctx.HTML(http.StatusOK, "user.html", templateParams{
		Email: session.Get("email"),
	})
}

type AddProject struct {
	ProjectName string `form:"projectName"`
}

func CreateHandler(ctx *gin.Context) {
	accessToken := sessions.Default(ctx).Get("access_token").(string)
	tempClient, closer, err := server.Connect(&oauth2.Token{AccessToken: accessToken})
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer closer()

	addProject := &AddProject{}
	if err := ctx.Bind(addProject); err != nil {
		return
	}
	if addProject.ProjectName == "" {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	_, err = tempClient.AddProject(ctx, &pb.AddProjectRequest{ProjectName: addProject.ProjectName})
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.Redirect(http.StatusSeeOther, ctx.Request.URL.Path)
}
