package workspacefiles

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func Handler(ctx *gin.Context) {
	session := sessions.Default(ctx)
	type templateParams struct {
		Email interface{}
	}
	ctx.HTML(http.StatusOK, "workspacefiles.html", templateParams{
		Email: session.Get("email"),
	})
}
