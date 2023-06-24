package jamhubgrpc

import (
	"context"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jamenv"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc/serverauth"
)

func (s JamHub) CreateUser(ctx context.Context, in *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	err = s.db.CreateUser(in.GetUsername(), id)
	if err != nil {
		return nil, err
	}
	return &pb.CreateUserResponse{}, nil
}

func (s JamHub) Ping(ctx context.Context, in *pb.PingRequest) (*pb.PingResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if jamenv.Env() == jamenv.Local {
		return &pb.PingResponse{Username: id}, nil
	}

	username, err := s.db.Username(id)
	if err != nil {
		return nil, err
	}

	return &pb.PingResponse{Username: username}, nil
}
