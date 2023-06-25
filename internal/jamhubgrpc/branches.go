package jamhubgrpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/fastcdc"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc/serverauth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s JamHub) CreateWorkspace(ctx context.Context, in *pb.CreateWorkspaceRequest) (*pb.CreateWorkspaceResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	maxCommitId, err := s.oplocstorecommit.MaxCommitId(userId, in.GetProjectId())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	workspaceId, err := s.changestore.AddWorkspace(userId, in.GetProjectId(), in.GetWorkspaceName(), maxCommitId)
	if err != nil {
		return nil, err
	}

	return &pb.CreateWorkspaceResponse{
		WorkspaceId: workspaceId,
	}, nil
}

func (s JamHub) GetWorkspaceName(ctx context.Context, in *pb.GetWorkspaceNameRequest) (*pb.GetWorkspaceNameResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	workspaceName, err := s.changestore.GetWorkspaceNameById(userId, in.GetProjectId(), in.GetWorkspaceId())
	if err != nil {
		return nil, err
	}

	return &pb.GetWorkspaceNameResponse{
		WorkspaceName: workspaceName,
	}, nil
}

func (s JamHub) GetWorkspaceId(ctx context.Context, in *pb.GetWorkspaceIdRequest) (*pb.GetWorkspaceIdResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	workspaceId, err := s.changestore.GetWorkspaceIdByName(userId, in.GetProjectId(), in.GetWorkspaceName())
	if err != nil {
		return nil, err
	}

	return &pb.GetWorkspaceIdResponse{
		WorkspaceId: workspaceId,
	}, nil
}

func (s JamHub) GetWorkspaceCurrentChange(ctx context.Context, in *pb.GetWorkspaceCurrentChangeRequest) (*pb.GetWorkspaceCurrentChangeResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	changeId, err := s.oplocstoreworkspace.MaxChangeId(userId, in.GetProjectId(), in.GetWorkspaceId())
	if err != nil {
		return nil, err
	}

	return &pb.GetWorkspaceCurrentChangeResponse{
		ChangeId: changeId,
	}, nil
}

func (s JamHub) ListWorkspaces(ctx context.Context, in *pb.ListWorkspacesRequest) (*pb.ListWorkspacesResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	workspaces, err := s.changestore.ListWorkspaces(userId, in.GetProjectId())
	if err != nil {
		return nil, err
	}

	return &pb.ListWorkspacesResponse{
		Workspaces: workspaces,
	}, nil
}

func (s JamHub) WriteWorkspaceOperationsStream(srv pb.JamHub_WriteWorkspaceOperationsStreamServer) error {
	userId, err := serverauth.ParseIdFromCtx(srv.Context())
	if err != nil {
		return err
	}

	var projectOwner string
	var projectId, workspaceId, changeId, operationProject uint64
	pathHashToOpLocs := make(map[string][]*pb.WorkspaceOperationLocations_OperationLocation, 0)
	for {
		in, err := srv.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		projectId = in.GetProjectId()
		workspaceId = in.GetWorkspaceId()
		changeId = in.GetChangeId()
		if operationProject == 0 {
			owner, err := s.db.GetProjectOwner(projectId)
			if err != nil {
				return err
			}
			if userId != owner {
				return status.Errorf(codes.Unauthenticated, "unauthorized")
			}
			projectOwner = owner
			operationProject = projectId
		}

		if operationProject != projectId {
			return status.Errorf(codes.Unauthenticated, "unauthorized")
		}

		pathHash := in.GetPathHash()

		var chunkHash *pb.ChunkHash
		var workspaceOffset, workspaceLength, commitOffset, commitLength uint64
		if in.GetOp().GetType() == pb.Operation_OpData {
			workspaceOffset, workspaceLength, err = s.opdatastoreworkspace.Write(userId, projectId, workspaceId, pathHash, in.GetOp())
			if err != nil {
				return err
			}
			chunkHash = &pb.ChunkHash{
				Offset: in.GetOp().GetChunk().GetOffset(),
				Length: in.GetOp().GetChunk().GetLength(),
				Hash:   in.GetOp().GetChunk().GetHash(),
			}
		} else {
			chunkHash = &pb.ChunkHash{
				Offset: in.GetOp().GetChunkHash().GetOffset(),
				Length: in.GetOp().GetChunkHash().GetLength(),
				Hash:   in.GetOp().GetChunkHash().GetHash(),
			}
		}

		if in.GetOp().GetType() == pb.Operation_OpBlock {
			opLocs, err := s.oplocstoreworkspace.ListOperationLocations(projectOwner, projectId, workspaceId, changeId-1, pathHash)
			if err != nil {
				return err
			}
			for _, loc := range opLocs.GetOpLocs() {
				if loc.GetChunkHash().GetHash() == in.GetOp().GetChunkHash().GetHash() {
					workspaceOffset = loc.GetOffset()
					workspaceLength = loc.GetLength()
					break
				}
			}

			if workspaceOffset == 0 && workspaceLength == 0 {
				commitId, err := s.changestore.GetWorkspaceBaseCommitId(projectOwner, projectId, workspaceId)
				if err != nil {
					return err
				}

				commitOpLocs, err := s.oplocstorecommit.ListOperationLocations(projectOwner, projectId, commitId, pathHash)
				if err != nil {
					return err
				}
				fmt.Println("HERE", commitOpLocs, projectOwner, projectId, commitId, pathHash)
				for _, loc := range commitOpLocs.GetOpLocs() {
					if loc.GetChunkHash().GetHash() == in.GetOp().GetChunkHash().GetHash() {
						commitOffset = loc.GetOffset()
						commitLength = loc.GetLength()
						break
					}
				}

				if commitOffset == 0 && commitLength == 0 {
					fmt.Println(projectOwner, projectId, commitId, pathHash, commitOffset, commitLength, commitOpLocs)
					log.Panic("Operation of type block but hash could not be found in workspace or commit")
				}
			}
		}

		operationLocation := &pb.WorkspaceOperationLocations_OperationLocation{
			Offset:       workspaceOffset,
			Length:       workspaceLength,
			CommitOffset: commitOffset,
			CommitLength: commitLength,
			ChunkHash:    chunkHash,
		}
		pathHashToOpLocs[string(pathHash)] = append(pathHashToOpLocs[string(pathHash)], operationLocation)
	}

	for pathHash, opLocs := range pathHashToOpLocs {
		err = s.oplocstoreworkspace.InsertOperationLocations(&pb.WorkspaceOperationLocations{
			ProjectId:   projectId,
			OwnerId:     projectOwner,
			WorkspaceId: workspaceId,
			ChangeId:    changeId,
			PathHash:    []byte(pathHash),
			OpLocs:      opLocs,
		})
		if err != nil {
			return err
		}
	}

	return srv.SendAndClose(&pb.WriteOperationStreamResponse{})
}

func (s JamHub) ReadWorkspaceChunkHashes(ctx context.Context, in *pb.ReadWorkspaceChunkHashesRequest) (*pb.ReadWorkspaceChunkHashesResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		if in.GetProjectId() != 1 {
			return nil, err
		}
	}

	targetBuffer, err := s.regenWorkspaceFile(userId, in.GetProjectId(), in.GetWorkspaceId(), in.GetChangeId(), in.GetPathHash())
	if err != nil {
		return nil, err
	}

	targetChunker, err := fastcdc.NewChunker(targetBuffer, fastcdc.Options{
		AverageSize: 1024 * 64,
		Seed:        84372,
	})
	if err != nil {
		return nil, err
	}
	sig := make([]*pb.ChunkHash, 0)
	err = targetChunker.CreateSignature(func(ch *pb.ChunkHash) error {
		sig = append(sig, ch)
		return nil
	})
	return &pb.ReadWorkspaceChunkHashesResponse{
		ChunkHashes: sig,
	}, err
}

func (s JamHub) regenWorkspaceFile(userId string, projectId, workspaceId, changeId uint64, pathHash []byte) (*bytes.Reader, error) {
	commitId, err := s.changestore.GetWorkspaceBaseCommitId(userId, projectId, workspaceId)
	if err != nil {
		return nil, err
	}

	committedFileReader, err := s.regenCommittedFile(userId, projectId, commitId, pathHash)
	if err != nil {
		return nil, err
	}

	var operationLocations *pb.WorkspaceOperationLocations
	for i := int(changeId); i >= 0 && operationLocations == nil; i-- {
		operationLocations, err = s.oplocstoreworkspace.ListOperationLocations(userId, projectId, workspaceId, uint64(i), pathHash)
		if err != nil {
			return nil, err
		}
	}
	if operationLocations == nil {
		return committedFileReader, nil
	}

	ops := make(chan *pb.Operation)
	go func() {
		for _, loc := range operationLocations.GetOpLocs() {
			if loc.GetCommitLength() != 0 {
				op, err := s.opdatastorecommit.Read(userId, projectId, pathHash, loc.GetCommitOffset(), loc.GetCommitLength())
				if err != nil {
					log.Panic(err)
				}
				ops <- op
			} else {
				op, err := s.opdatastoreworkspace.Read(userId, projectId, workspaceId, pathHash, loc.GetOffset(), loc.GetLength())
				if err != nil {
					log.Panic(err)
				}
				ops <- op
			}
		}
		close(ops)
	}()
	result := new(bytes.Buffer)
	chunker, err := fastcdc.NewChunker(committedFileReader, fastcdc.Options{
		AverageSize: 1024 * 64,
		Seed:        84372,
	})
	if err != nil {
		log.Panic(err)
	}
	err = chunker.ApplyDelta(result, committedFileReader, ops)
	if err != nil {
		log.Panic(err)
	}

	return bytes.NewReader(result.Bytes()), nil
}

func (s JamHub) ReadWorkspaceFile(in *pb.ReadWorkspaceFileRequest, srv pb.JamHub_ReadWorkspaceFileServer) error {
	userId, err := serverauth.ParseIdFromCtx(srv.Context())
	if err != nil {
		return err
	}

	changeId := in.GetChangeId()
	if changeId == 0 {
		maxChangeId, err := s.oplocstoreworkspace.MaxChangeId(userId, in.GetProjectId(), in.GetWorkspaceId())
		if err != nil {
			return err
		}
		changeId = maxChangeId
	}

	sourceBuffer, err := s.regenWorkspaceFile(userId, in.GetProjectId(), in.GetWorkspaceId(), changeId, in.GetPathHash())
	if err != nil {
		return err
	}

	sourceChunker, err := fastcdc.NewChunker(sourceBuffer, fastcdc.Options{
		AverageSize: 1024 * 64,
		Seed:        84372,
	})
	if err != nil {
		return err
	}

	opsOut := make(chan *pb.Operation)
	tot := 0
	go func() {
		var blockCt, dataCt, bytes int
		defer close(opsOut)
		err := sourceChunker.CreateDelta(in.GetChunkHashes(), func(op *pb.Operation) error {
			tot += int(op.Chunk.GetLength()) + int(op.ChunkHash.GetLength())
			switch op.Type {
			case pb.Operation_OpBlock:
				blockCt++
			case pb.Operation_OpData:
				b := make([]byte, len(op.Chunk.Data))
				copy(b, op.Chunk.Data)
				op.Chunk.Data = b
				dataCt++
				bytes += len(op.Chunk.Data)
			}
			opsOut <- op
			return nil
		})
		if err != nil {
			panic(err)
		}
	}()

	for op := range opsOut {
		err = srv.Send(&pb.WorkspaceFileOperation{
			WorkspaceId: in.WorkspaceId,
			ProjectId:   in.GetProjectId(),
			PathHash:    in.GetPathHash(),
			Op:          op,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s JamHub) DeleteWorkspace(ctx context.Context, in *pb.DeleteWorkspaceRequest) (*pb.DeleteWorkspaceResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	err = s.opdatastoreworkspace.DeleteWorkspace(userId, in.GetProjectId(), in.GetWorkspaceId())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	err = s.oplocstoreworkspace.DeleteWorkspace(userId, in.GetProjectId(), in.GetWorkspaceId())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	err = s.changestore.DeleteWorkspace(userId, in.GetProjectId(), in.GetWorkspaceId())
	if err != nil {
		return nil, err
	}

	return &pb.DeleteWorkspaceResponse{}, nil
}
