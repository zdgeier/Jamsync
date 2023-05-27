package branchfile

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zdgeier/jamsync/internal/jamenv"
)

func Handler(ctx *gin.Context) {
	session := sessions.Default(ctx)
	type templateParams struct {
		Email  interface{}
		IsProd bool
	}
	ctx.HTML(http.StatusOK, "branchfile.html", templateParams{
		Email:  session.Get("email"),
		IsProd: jamenv.Env() == jamenv.Prod,
	})
}
