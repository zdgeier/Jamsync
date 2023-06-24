package committedfile

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zdgeier/jamhub/internal/jamenv"
)

func Handler(ctx *gin.Context) {
	session := sessions.Default(ctx)
	type templateParams struct {
		Email  interface{}
		IsProd bool
	}
	ctx.HTML(http.StatusOK, "committedfile.html", templateParams{
		Email:  session.Get("email"),
		IsProd: jamenv.Env() == jamenv.Prod,
	})
}
