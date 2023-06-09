package authenticator

import (
	"context"
	"errors"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	cv "github.com/nirasan/go-oauth-pkce-code-verifier"
	"github.com/zdgeier/jamhub/internal/jamenv"
	"golang.org/x/oauth2"
)

type Authenticator struct {
	*oidc.Provider
	oauth2.Config
	CodeVerifier *cv.CodeVerifier
}

func New() (*Authenticator, error) {
	if jamenv.Env() == jamenv.Local {
		return nil, nil
	}
	provider, err := oidc.NewProvider(
		context.Background(),
		"https://"+os.Getenv("AUTH0_DOMAIN")+"/",
	)
	if err != nil {
		return nil, err
	}

	conf := oauth2.Config{
		ClientID:     os.Getenv("AUTH0_CLIENT_ID"),
		ClientSecret: os.Getenv("AUTH0_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("AUTH0_CALLBACK_URL"),
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile email add:projects"},
	}

	codeVerifier, err := cv.CreateCodeVerifier()
	if err != nil {
		return nil, err
	}

	return &Authenticator{
		Provider:     provider,
		Config:       conf,
		CodeVerifier: codeVerifier,
	}, nil
}

func (a *Authenticator) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token field in oauth2 token")
	}

	oidcConfig := &oidc.Config{
		ClientID: a.ClientID,
	}

	return a.Verifier(oidcConfig).Verify(ctx, rawIDToken)
}
