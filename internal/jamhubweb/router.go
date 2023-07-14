package jamhubweb

import (
	"encoding/gob"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/handlers"

	"github.com/zdgeier/jamhub/internal/jamenv"
	"github.com/zdgeier/jamhub/internal/jamhubweb/api"
	"github.com/zdgeier/jamhub/internal/jamhubweb/authenticator"
	"github.com/zdgeier/jamhub/internal/jamhubweb/callback"
	"github.com/zdgeier/jamhub/internal/jamhubweb/committedfile"
	"github.com/zdgeier/jamhub/internal/jamhubweb/committedfiles"
	"github.com/zdgeier/jamhub/internal/jamhubweb/download"
	"github.com/zdgeier/jamhub/internal/jamhubweb/login"
	"github.com/zdgeier/jamhub/internal/jamhubweb/logout"
	"github.com/zdgeier/jamhub/internal/jamhubweb/middleware"
	"github.com/zdgeier/jamhub/internal/jamhubweb/projectinfo"
	"github.com/zdgeier/jamhub/internal/jamhubweb/userprojects"
	"github.com/zdgeier/jamhub/internal/jamhubweb/workspacefile"
	"github.com/zdgeier/jamhub/internal/jamhubweb/workspacefiles"
)

type templateParams struct {
	Email interface{}
}

func New(auth *authenticator.Authenticator) http.Handler {
	if jamenv.Env() == jamenv.Prod {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	// To store custom types in our cookies,
	// we must first register them using gob.Register
	gob.Register(map[string]interface{}{})
	gob.Register(time.Time{})

	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.Sessions("auth-session", store))

	router.Static("/public", "public")

	router.SetFuncMap(template.FuncMap{
		"args": func(kvs ...interface{}) (map[string]interface{}, error) {
			if len(kvs)%2 != 0 {
				return nil, errors.New("args requires even number of arguments")
			}
			m := make(map[string]interface{})
			for i := 0; i < len(kvs); i += 2 {
				s, ok := kvs[i].(string)
				if !ok {
					return nil, errors.New("even args to args must be strings")
				}
				m[s] = kvs[i+1]
			}
			return m, nil
		},
	})
	router.LoadHTMLGlob("template/*")

	router.GET("/", middleware.Reauthenticate, func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		ctx.HTML(http.StatusOK, "home.html", templateParams{
			Email: session.Get("email"),
		})
	})
	router.GET("/about", middleware.Reauthenticate, func(ctx *gin.Context) {
		ctx.Redirect(http.StatusMovedPermanently, "/")
	})
	router.GET("/beta", middleware.Reauthenticate, func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		ctx.HTML(http.StatusOK, "beta.html", templateParams{
			Email: session.Get("email"),
		})
	})
	router.GET("/browse", middleware.Reauthenticate, func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		ctx.HTML(http.StatusOK, "browse.html", templateParams{
			Email: session.Get("email"),
		})
	})
	router.GET("/abuse", middleware.Reauthenticate, func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		ctx.HTML(http.StatusOK, "abuse.html", templateParams{
			Email: session.Get("email"),
		})
	})
	router.GET("/terms", middleware.Reauthenticate, func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		ctx.HTML(http.StatusOK, "terms.html", templateParams{
			Email: session.Get("email"),
		})
	})
	router.GET("/privacy", middleware.Reauthenticate, func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		ctx.HTML(http.StatusOK, "privacy.html", templateParams{
			Email: session.Get("email"),
		})
	})
	router.GET("/favicon.ico", func(ctx *gin.Context) {
		ctx.Header("Content-Type", "image/svg+xml")
		ctx.File("public/favicon.svg")
	})
	router.GET("/favicon.svg", func(ctx *gin.Context) {
		ctx.Header("Content-Type", "image/svg+xml")
		ctx.File("public/favicon.svg")
	})
	router.GET("/robots.txt", func(ctx *gin.Context) {
		ctx.Header("Content-Type", "text/plain")
		ctx.File("public/robots.txt")
	})
	router.GET("/.well-known/acme-challenge/4TqqfL3ONUUMG7OrFYsNy_UzyelKciboqYsmvRamJPc", func(ctx *gin.Context) {
		ctx.Header("Content-Type", "text/plain")
		ctx.File("public/4TqqfL3ONUUMG7OrFYsNy_UzyelKciboqYsmvRamJPc")
	})

	router.GET("/login", login.Handler(auth))
	router.GET("/callback", callback.Handler(auth))
	router.GET("/logout", logout.Handler)

	router.GET("/api/users/:username", api.UserProjectsHandler())
	router.GET("/api/commits/:owner/:projectName", api.GetProjectCurrentCommitHandler())
	router.GET("/api/collaborators/:owner/:projectName", api.GetCollaboratorsHandler())
	router.PUT("/api/collaborators/:owner/:projectName", api.AddCollaboratorHandler())
	router.GET("/api/workspaces/:owner/:projectName", api.GetWorkspacesHandler())
	router.GET("/api/workspaces/:owner/:projectName/:workspaceName", api.GetWorkspaceInfoHandler())
	router.GET("/api/committedfiles/:owner/:projectName/:commitId", api.ProjectBrowseCommitHandler())
	router.GET("/api/committedfile/:owner/:projectName/:commitId", api.GetFileCommitHandler())
	router.GET("/api/workspacefiles/:owner/:projectName/:workspaceId", api.ProjectBrowseWorkspaceHandler())
	router.GET("/api/workspacefile/:owner/:projectName/:workspaceId", api.GetFileWorkspaceHandler())

	router.GET("/:username/projects", middleware.IsAuthenticated, middleware.Reauthenticate, userprojects.Handler)
	router.GET("/:username/:project/workspacefile/:workspaceName/*path", middleware.IsAuthenticated, middleware.Reauthenticate, workspacefile.Handler)
	router.GET("/:username/:project/committedfile/*path", middleware.IsAuthenticated, middleware.Reauthenticate, committedfile.Handler)
	router.GET("/:username/:project/committedfiles/*path", middleware.IsAuthenticated, middleware.Reauthenticate, committedfiles.Handler)
	router.GET("/:username/:project/projectinfo", middleware.IsAuthenticated, middleware.Reauthenticate, projectinfo.Handler)
	router.GET("/:username/:project/workspacefiles/:workspaceName/*path", middleware.IsAuthenticated, middleware.Reauthenticate, workspacefiles.Handler)
	router.GET("/download", middleware.IsAuthenticated, middleware.Reauthenticate, download.Handler)
	return MaxAge(handlers.CompressHandler(router))
}

// MaxAge sets expire headers based on extension
func MaxAge(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var age time.Duration
		ext := filepath.Ext(r.URL.String())

		// Timings are based on github.com/h5bp/server-configs-nginx

		switch ext {
		case ".rss", ".atom":
			age = time.Hour / time.Second
		case ".css", ".js":
			age = (time.Hour * 24 * 365) / time.Second
		case ".jpg", ".jpeg", ".gif", ".png", ".ico", ".cur", ".gz", ".svg", ".svgz", ".mp4", ".ogg", ".ogv", ".webm", ".htc", ".woff2":
			age = (time.Hour * 24 * 30) / time.Second
		default:
			age = 0
		}

		if ext == ".woff2" {
			w.Header().Add("Access-Control-Allow-Origin", "https://jamsync.us.auth0.com")
		}

		if age > 0 {
			w.Header().Add("Cache-Control", fmt.Sprintf("max-age=%d, public, must-revalidate, proxy-revalidate", age))
		}

		h.ServeHTTP(w, r)
	})
}
