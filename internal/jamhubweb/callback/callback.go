package callback

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jamenv"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"github.com/zdgeier/jamhub/internal/jamhubweb/authenticator"
	"golang.org/x/oauth2"
)

func Handler(auth *authenticator.Authenticator) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if jamenv.Env() == jamenv.Local {
			session := sessions.Default(ctx)
			email := "test@jamhub.dev"
			emailQuery, _ := ctx.GetQuery("email")
			if emailQuery != "" {
				email = emailQuery
			}
			session.Set("access_token", email)
			session.Set("email", email)
			if err := session.Save(); err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
			token := &oauth2.Token{AccessToken: email}
			tempClient, closer, err := jamhubgrpc.Connect(token)
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
			defer closer()

			_, err = tempClient.CreateUser(ctx, &pb.CreateUserRequest{
				Username: email,
			})
			if err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}

			ctx.Redirect(http.StatusTemporaryRedirect, email+"/projects")
			return
		}

		session := sessions.Default(ctx)
		if ctx.Query("state") != session.Get("state") {
			ctx.String(http.StatusBadRequest, "Invalid state parameter.")
			return
		}

		// Exchange an authorization code for a token.
		token, err := auth.Exchange(ctx.Request.Context(), ctx.Query("code"))
		if err != nil {
			ctx.String(http.StatusUnauthorized, "Failed to exchange an authorization code for a token.", err)
			return
		}
		idToken, err := auth.VerifyIDToken(ctx.Request.Context(), token)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to verify ID Token.")
			return
		}

		var profile map[string]interface{}
		if err := idToken.Claims(&profile); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		session.Set("exp", token.Expiry)
		session.Set("access_token", token.AccessToken)
		session.Set("email", profile["email"])
		if err := session.Save(); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		// Use a temp client here so we dont have to do anything complicated to validate the user
		// We'll do user validation on the API side so users can only create their own accounts
		tempClient, closer, err := jamhubgrpc.Connect(token)
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer closer()

		_, err = tempClient.CreateUser(ctx, &pb.CreateUserRequest{
			Username: profile["email"].(string),
		})
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		ctx.Redirect(http.StatusTemporaryRedirect, "/"+profile["email"].(string)+"/projects")
	}
}
