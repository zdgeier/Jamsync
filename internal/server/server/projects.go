package server

import (
	"context"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/server/serverauth"
)

func (s JamsyncServer) AddProject(ctx context.Context, in *pb.AddProjectRequest) (*pb.AddProjectResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	projectId, err := s.db.AddProject(in.GetProjectName(), id)
	if err != nil {
		return nil, err
	}

	return &pb.AddProjectResponse{
		ProjectId: projectId,
	}, nil
}

func (s JamsyncServer) ListUserProjects(ctx context.Context, in *pb.ListUserProjectsRequest) (*pb.ListUserProjectsResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	projects, err := s.db.ListUserProjects(id)
	if err != nil {
		return nil, err
	}

	projectsPb := make([]*pb.ListUserProjectsResponse_Project, len(projects))
	for i := range projectsPb {
		projectsPb[i] = &pb.ListUserProjectsResponse_Project{Name: projects[i].Name, Id: projects[i].Id}
	}

	return &pb.ListUserProjectsResponse{Projects: projectsPb}, nil
}

func (s JamsyncServer) GetProjectId(ctx context.Context, in *pb.GetProjectIdRequest) (*pb.GetProjectIdResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	projectId, err := s.db.GetProjectId(in.GetProjectName(), userId)
	if err != nil {
		return nil, err
	}

	return &pb.GetProjectIdResponse{ProjectId: projectId}, nil
}

func (s JamsyncServer) DeleteProject(ctx context.Context, in *pb.DeleteProjectRequest) (*pb.DeleteProjectResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	projectName := in.GetProjectName()
	if in.GetProjectId() != 0 {
		projectName, err = s.db.GetProjectName(in.GetProjectId(), id)
		if err != nil {
			return nil, err
		}
	}

	projectId, err := s.db.DeleteProject(projectName, id)
	if err != nil {
		return nil, err
	}
	err = s.changestore.DeleteProject(projectId, id)
	if err != nil {
		return nil, err
	}
	err = s.oplocstorebranch.DeleteProject(id, projectId)
	if err != nil {
		return nil, err
	}
	err = s.oplocstorecommit.DeleteProject(id, projectId)
	if err != nil {
		return nil, err
	}
	err = s.opdatastorebranch.DeleteProject(id, projectId)
	if err != nil {
		return nil, err
	}
	err = s.opdatastorecommit.DeleteProject(id, projectId)
	if err != nil {
		return nil, err
	}
	return &pb.DeleteProjectResponse{
		ProjectId:   projectId,
		ProjectName: projectName,
	}, nil
}
