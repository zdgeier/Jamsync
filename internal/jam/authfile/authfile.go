package authfile

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jamhub/clientauth"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
	"golang.org/x/oauth2"
)

type AuthFile struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

func Authorize() (AuthFile, error) {
	rawFile, err := os.ReadFile(authPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return AuthFile{}, err
	}

	if errors.Is(err, os.ErrNotExist) {
		token, err := clientauth.AuthorizeUser()
		if err != nil {
			log.Panic(err)
		}

		apiClient, closer, err := jamhubgrpc.Connect(&oauth2.Token{
			AccessToken: string(token),
		})
		if err != nil {
			log.Panic(err)
		}
		defer closer()

		resp, err := apiClient.Ping(context.Background(), &pb.PingRequest{})
		if err != nil {
			log.Panic(err)
		}

		authFile := AuthFile{
			Token:    token,
			Username: resp.GetUsername(),
		}

		data, err := json.Marshal(authFile)
		if err != nil {
			log.Panic(err)
		}

		err = os.WriteFile(authPath(), data, fs.ModePerm)
		if err != nil {
			log.Panic(err)
		}
		return authFile, nil
	}

	authFile := AuthFile{}
	err = json.Unmarshal(rawFile, &authFile)
	if err != nil {
		return authFile, err
	}

	apiClient, closer, err := jamhubgrpc.Connect(&oauth2.Token{
		AccessToken: string(authFile.Token),
	})
	if err != nil {
		log.Panic(err)
	}
	defer closer()

	_, err = apiClient.Ping(context.Background(), &pb.PingRequest{})
	if err != nil {
		// If outdated token
		err := os.Remove(authPath())
		if err != nil {
			return authFile, err
		}
		Authorize()
	}

	return authFile, nil
}

func Logout() error {
	return os.Remove(authPath())
}

func authPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return path.Join(home, ".jamhubauth")
}
