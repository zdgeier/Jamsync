package serverauth

import (
	"context"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/zdgeier/jamhub/internal/jamenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var provider *jwks.CachingProvider

var (
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errInvalidToken    = status.Errorf(codes.Unauthenticated, "invalid token")
)

func EnsureValidToken(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if jamenv.Env() == jamenv.Local {
		return handler(ctx, req)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errMissingMetadata
	}

	authorizationHeader := md.Get("authorization")
	if len(authorizationHeader) < 1 || authorizationHeader[0] == "" {
		return nil, errInvalidToken
	}

	token := strings.TrimPrefix(authorizationHeader[0], "Bearer ")
	_, err := ensureValidToken(token)
	if err != nil {
		return nil, err
	}

	return handler(ctx, req)
}

// EnsureValidToken is a middleware that will check the validity of our JWT.
func ensureValidToken(token string) (*validator.ValidatedClaims, error) {
	issuerURL, err := url.Parse("https://" + os.Getenv("AUTH0_DOMAIN") + "/")
	if err != nil {
		log.Panicf("Failed to parse the issuer url: %v", err)
	}
	if provider == nil {
		provider = jwks.NewCachingProvider(issuerURL, 5*time.Minute)
	}

	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{"api.jamsync.dev"},
		validator.WithCustomClaims(
			func() validator.CustomClaims {
				return &CustomClaims{}
			},
		),
		validator.WithAllowedClockSkew(time.Minute),
	)
	if err != nil {
		log.Panicf("Failed to set up the jwt validator")
	}

	rawValidatedClaims, err := jwtValidator.ValidateToken(context.Background(), token)
	if err != nil {
		return nil, err
	}

	return rawValidatedClaims.(*validator.ValidatedClaims), nil
}

// CustomClaims contains custom data we want from the token.
type CustomClaims struct {
	Scope string `json:"scope"`
}

// Validate does nothing for this example, but we need
// it to satisfy validator.CustomClaims interface.
func (c CustomClaims) Validate(ctx context.Context) error {
	return nil
}

func (c CustomClaims) HasScope(expectedScope string) bool {
	result := strings.Split(c.Scope, " ")
	for i := range result {
		if result[i] == expectedScope {
			return true
		}
	}

	return false
}

func ParseIdFromCtx(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errMissingMetadata
	}

	authorizationHeader := md.Get("authorization")
	if len(authorizationHeader) < 1 || authorizationHeader[0] == "" {
		return "", errInvalidToken
	}

	token := strings.TrimPrefix(authorizationHeader[0], "Bearer ")
	if jamenv.Env() == jamenv.Local {
		return token, nil
	}
	validatedClaims, err := ensureValidToken(token)
	if err != nil {
		return "", err
	}

	return validatedClaims.RegisteredClaims.Subject, nil
}
