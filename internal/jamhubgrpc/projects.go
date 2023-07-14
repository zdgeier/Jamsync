package jamhubgrpc

import (
	"context"
	"errors"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc/serverauth"
)

func (s JamHub) GetProjectName(ctx context.Context, in *pb.GetProjectNameRequest) (*pb.GetProjectNameResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	projectName, err := s.db.GetProjectName(in.GetProjectId(), id)
	if err != nil {
		return nil, err
	}

	return &pb.GetProjectNameResponse{
		ProjectName: projectName,
	}, nil
}

func (s JamHub) AddProject(ctx context.Context, in *pb.AddProjectRequest) (*pb.AddProjectResponse, error) {
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

func (s JamHub) GetCollaborators(ctx context.Context, in *pb.GetCollaboratorsRequest) (*pb.GetCollaboratorsResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	owner, err := s.db.GetProjectOwner(in.GetProjectId())
	if err != nil {
		return nil, err
	}

	if id != owner {
		return nil, errors.New("not the owner of this project")
	}

	usernames, err := s.db.ListCollaborators(in.GetProjectId())
	if err != nil {
		return nil, err
	}

	return &pb.GetCollaboratorsResponse{
		Usernames: usernames,
	}, nil
}

func (s JamHub) AddCollaborator(ctx context.Context, in *pb.AddCollaboratorRequest) (*pb.AddCollaboratorResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	owner, err := s.db.GetProjectOwner(in.GetProjectId())
	if err != nil {
		return nil, err
	}

	if id != owner {
		return nil, errors.New("not the owner of this project")
	}

	err = s.db.AddCollaborator(in.GetProjectId(), in.GetUsername())
	if err != nil {
		return nil, err
	}

	return &pb.AddCollaboratorResponse{}, nil
}

func (s JamHub) ListUserProjects(ctx context.Context, in *pb.ListUserProjectsRequest) (*pb.ListUserProjectsResponse, error) {
	id, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if id != in.GetOwner() {
		return nil, errors.New("unauthorized: cannot list other user projects for now")
	}

	projects, err := s.db.ListProjectsOwned(id)
	if err != nil {
		return nil, err
	}

	collabProjects, err := s.db.ListProjectsAsCollaborator(id)
	if err != nil {
		return nil, err
	}

	projectsPb := make([]*pb.ListUserProjectsResponse_Project, 0, len(projects)+len(collabProjects))
	for _, p := range projects {
		projectsPb = append(projectsPb, &pb.ListUserProjectsResponse_Project{Name: p.Name, Id: p.Id})
	}
	for _, p := range collabProjects {
		projectsPb = append(projectsPb, &pb.ListUserProjectsResponse_Project{Name: p.Name, Id: p.Id})
	}

	return &pb.ListUserProjectsResponse{Projects: projectsPb}, nil
}

func (s JamHub) ProjectAccessible(owner string, projectName string, currentUsername string) (bool, error) {
	projectId, err := s.db.GetProjectId(projectName, owner)
	if err != nil {
		return false, err
	}

	projectOwner, err := s.db.GetProjectOwner(projectId)
	if err != nil {
		return false, err
	}

	if owner == projectOwner && owner == currentUsername {
		return true, nil
	}

	if s.db.HasCollaborator(projectId, currentUsername) {
		return true, nil
	}

	return false, nil
}

func (s JamHub) GetProjectId(ctx context.Context, in *pb.GetProjectIdRequest) (*pb.GetProjectIdResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	canAccess, err := s.ProjectAccessible(, in.GetProjectName(), userId)
	if err != nil {
		return nil, err
	} else if !canAccess {
		return nil, errors.New("cannot access project")
	}

	projectId, err := s.db.GetProjectId(in.GetProjectName(), userId)
	if err != nil {
		return nil, err
	}

	return &pb.GetProjectIdResponse{ProjectId: projectId}, nil
}

func (s JamHub) DeleteProject(ctx context.Context, in *pb.DeleteProjectRequest) (*pb.DeleteProjectResponse, error) {
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
	err = s.oplocstoreworkspace.DeleteProject(id, projectId)
	if err != nil {
		return nil, err
	}
	err = s.oplocstorecommit.DeleteProject(id, projectId)
	if err != nil {
		return nil, err
	}
	err = s.opdatastoreworkspace.DeleteProject(id, projectId)
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
