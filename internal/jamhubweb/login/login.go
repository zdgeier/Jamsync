package login

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zdgeier/jamhub/internal/jamenv"
	"github.com/zdgeier/jamhub/internal/jamhubweb/authenticator"
	"golang.org/x/oauth2"
)

func Handler(auth *authenticator.Authenticator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if jamenv.Env() == jamenv.Local {
			ctx.Redirect(http.StatusTemporaryRedirect, "/callback")
			return
		}

		state, err := generateRandomState()
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		session := sessions.Default(ctx)
		session.Set("state", state)
		if err := session.Save(); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		ctx.Redirect(http.StatusTemporaryRedirect, auth.AuthCodeURL(state, oauth2.SetAuthURLParam("audience", "api.jamsync.dev")))
	}
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	state := base64.StdEncoding.EncodeToString(b)

	return state, nil
}
