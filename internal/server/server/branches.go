package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/fastcdc"
	"github.com/zdgeier/jamsync/internal/server/serverauth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s JamsyncServer) CreateBranch(ctx context.Context, in *pb.CreateBranchRequest) (*pb.CreateBranchResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	maxCommitId, err := s.oplocstorecommit.MaxCommitId(userId, in.GetProjectId())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	branchId, err := s.changestore.AddBranch(userId, in.GetProjectId(), in.GetBranchName(), maxCommitId)
	if err != nil {
		return nil, err
	}

	return &pb.CreateBranchResponse{
		BranchId: branchId,
	}, nil
}

func (s JamsyncServer) GetBranchName(ctx context.Context, in *pb.GetBranchNameRequest) (*pb.GetBranchNameResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	branchName, err := s.changestore.GetBranchNameById(userId, in.GetProjectId(), in.GetBranchId())
	if err != nil {
		return nil, err
	}

	return &pb.GetBranchNameResponse{
		BranchName: branchName,
	}, nil
}

func (s JamsyncServer) GetBranchId(ctx context.Context, in *pb.GetBranchIdRequest) (*pb.GetBranchIdResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	branchId, err := s.changestore.GetBranchIdByName(userId, in.GetProjectId(), in.GetBranchName())
	if err != nil {
		return nil, err
	}

	return &pb.GetBranchIdResponse{
		BranchId: branchId,
	}, nil
}

func (s JamsyncServer) GetBranchCurrentChange(ctx context.Context, in *pb.GetBranchCurrentChangeRequest) (*pb.GetBranchCurrentChangeResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	changeId, err := s.oplocstorebranch.MaxChangeId(userId, in.GetProjectId(), in.GetBranchId())
	if err != nil {
		return nil, err
	}

	return &pb.GetBranchCurrentChangeResponse{
		ChangeId: changeId,
	}, nil
}

func (s JamsyncServer) ListBranches(ctx context.Context, in *pb.ListBranchesRequest) (*pb.ListBranchesResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	branches, err := s.changestore.ListBranches(userId, in.GetProjectId())
	if err != nil {
		return nil, err
	}

	return &pb.ListBranchesResponse{
		Branches: branches,
	}, nil
}

func (s JamsyncServer) WriteBranchOperationsStream(srv pb.JamsyncAPI_WriteBranchOperationsStreamServer) error {
	userId, err := serverauth.ParseIdFromCtx(srv.Context())
	if err != nil {
		return err
	}

	var projectOwner string
	var projectId, branchId, changeId, operationProject uint64
	pathHashToOpLocs := make(map[string][]*pb.BranchOperationLocations_OperationLocation, 0)
	for {
		in, err := srv.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		projectId = in.GetProjectId()
		branchId = in.GetBranchId()
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
		var branchOffset, branchLength, commitOffset, commitLength uint64
		if in.GetOp().GetType() == pb.Operation_OpData {
			branchOffset, branchLength, err = s.opdatastorebranch.Write(userId, projectId, branchId, pathHash, in.GetOp())
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
			opLocs, err := s.oplocstorebranch.ListOperationLocations(projectOwner, projectId, branchId, changeId-1, pathHash)
			if err != nil {
				return err
			}
			for _, loc := range opLocs.GetOpLocs() {
				if loc.GetChunkHash().GetHash() == in.GetOp().GetChunkHash().GetHash() {
					branchOffset = loc.GetOffset()
					branchLength = loc.GetLength()
					break
				}
			}

			if branchOffset == 0 && branchLength == 0 {
				commitId, err := s.changestore.GetBranchBaseCommitId(projectOwner, projectId, branchId)
				if err != nil {
					return err
				}

				commitOpLocs, err := s.oplocstorecommit.ListOperationLocations(projectOwner, projectId, commitId, pathHash)
				if err != nil {
					return err
				}
				for _, loc := range commitOpLocs.GetOpLocs() {
					if loc.GetChunkHash().GetHash() == in.GetOp().GetChunkHash().GetHash() {
						commitOffset = loc.GetOffset()
						commitLength = loc.GetLength()
						break
					}
				}

				if commitOffset == 0 && commitLength == 0 {
					fmt.Println(projectOwner, projectId, commitId, pathHash, commitOffset, commitLength, commitOpLocs)
					log.Panic("Operation of type block but hash could not be found in branch or commit")
				}
			}
		}

		operationLocation := &pb.BranchOperationLocations_OperationLocation{
			Offset:       branchOffset,
			Length:       branchLength,
			CommitOffset: commitOffset,
			CommitLength: commitLength,
			ChunkHash:    chunkHash,
		}
		pathHashToOpLocs[string(pathHash)] = append(pathHashToOpLocs[string(pathHash)], operationLocation)
	}

	for pathHash, opLocs := range pathHashToOpLocs {
		err = s.oplocstorebranch.InsertOperationLocations(&pb.BranchOperationLocations{
			ProjectId: projectId,
			OwnerId:   projectOwner,
			BranchId:  branchId,
			ChangeId:  changeId,
			PathHash:  []byte(pathHash),
			OpLocs:    opLocs,
		})
		if err != nil {
			return err
		}
	}

	return srv.SendAndClose(&pb.WriteOperationStreamResponse{})
}

func (s JamsyncServer) ReadBranchChunkHashes(ctx context.Context, in *pb.ReadBranchChunkHashesRequest) (*pb.ReadBranchChunkHashesResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		if in.GetProjectId() != 1 {
			return nil, err
		}
	}

	targetBuffer, err := s.regenBranchFile(userId, in.GetProjectId(), in.GetBranchId(), in.GetChangeId(), in.GetPathHash())
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
	return &pb.ReadBranchChunkHashesResponse{
		ChunkHashes: sig,
	}, err
}

func (s JamsyncServer) regenBranchFile(userId string, projectId, branchId, changeId uint64, pathHash []byte) (*bytes.Reader, error) {
	commitId, err := s.changestore.GetBranchBaseCommitId(userId, projectId, branchId)
	if err != nil {
		return nil, err
	}

	committedFileReader, err := s.regenCommittedFile(userId, projectId, commitId, pathHash)
	if err != nil {
		return nil, err
	}

	var operationLocations *pb.BranchOperationLocations
	for i := int(changeId); i >= 0 && operationLocations == nil; i-- {
		operationLocations, err = s.oplocstorebranch.ListOperationLocations(userId, projectId, branchId, uint64(i), pathHash)
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
				op, err := s.opdatastorebranch.Read(userId, projectId, branchId, pathHash, loc.GetOffset(), loc.GetLength())
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

func (s JamsyncServer) ReadBranchFile(in *pb.ReadBranchFileRequest, srv pb.JamsyncAPI_ReadBranchFileServer) error {
	userId, err := serverauth.ParseIdFromCtx(srv.Context())
	if err != nil {
		return err
	}

	changeId := in.GetChangeId()
	if changeId == 0 {
		maxChangeId, err := s.oplocstorebranch.MaxChangeId(userId, in.GetProjectId(), in.GetBranchId())
		if err != nil {
			return err
		}
		changeId = maxChangeId
	}

	sourceBuffer, err := s.regenBranchFile(userId, in.GetProjectId(), in.GetBranchId(), changeId, in.GetPathHash())
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
		err = srv.Send(&pb.BranchFileOperation{
			BranchId:  in.BranchId,
			ProjectId: in.GetProjectId(),
			PathHash:  in.GetPathHash(),
			Op:        op,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s JamsyncServer) DeleteBranch(ctx context.Context, in *pb.DeleteBranchRequest) (*pb.DeleteBranchResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	err = s.opdatastorebranch.DeleteBranch(userId, in.GetProjectId(), in.GetBranchId())
	if err != nil {
		return nil, err
	}

	err = s.oplocstorebranch.DeleteBranch(userId, in.GetProjectId(), in.GetBranchId())
	if err != nil {
		return nil, err
	}

	err = s.changestore.DeleteBranch(userId, in.GetProjectId(), in.GetBranchId())
	if err != nil {
		return nil, err
	}

	return &pb.DeleteBranchResponse{}, nil
}
