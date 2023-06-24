// platform/middleware/isAuthenticated.go

package middleware

import (
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zdgeier/jamhub/internal/jamenv"
)

func IsAuthenticated(ctx *gin.Context) {
	if jamenv.Env() == jamenv.Local {
		ctx.Next()
		return
	}

	if sessions.Default(ctx).Get("email") == nil {
		ctx.Redirect(http.StatusSeeOther, "/")
	} else {
		ctx.Next()
	}
}

func Reauthenticate(ctx *gin.Context) {
	if jamenv.Env() == jamenv.Local {
		ctx.Next()
		return
	}

	exp := sessions.Default(ctx).Get("exp")
	if exp != nil && exp.(time.Time).Before(time.Now()) {
		ctx.Redirect(http.StatusTemporaryRedirect, "/login")
	}
	ctx.Next()
}
