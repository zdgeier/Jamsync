package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/fastcdc"
	"github.com/zdgeier/jamsync/internal/server/serverauth"
)

func (s JamsyncServer) GetProjectCurrentCommit(ctx context.Context, in *pb.GetProjectCurrentCommitRequest) (*pb.GetProjectCurrentCommitResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Println("ID", in.ProjectId)
	commitId, err := s.oplocstorecommit.MaxCommitId(userId, in.ProjectId)
	if err != nil {
		return nil, err
	}
	fmt.Println(commitId)

	return &pb.GetProjectCurrentCommitResponse{
		CommitId: commitId,
	}, err
}

func (s JamsyncServer) ReadCommitChunkHashes(ctx context.Context, in *pb.ReadCommitChunkHashesRequest) (*pb.ReadCommitChunkHashesResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		if in.GetProjectId() != 1 {
			return nil, err
		}
	}

	targetBuffer, err := s.regenCommittedFile(userId, in.GetProjectId(), in.GetCommitId(), in.GetPathHash())
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
	return &pb.ReadCommitChunkHashesResponse{
		ChunkHashes: sig,
	}, err
}

func (s JamsyncServer) regenCommittedFile(userId string, projectId uint64, commitId uint64, pathHash []byte) (*bytes.Reader, error) {
	var err error
	var operationLocations *pb.CommitOperationLocations
	for i := int(commitId); i >= 0 && operationLocations == nil; i-- {
		operationLocations, err = s.oplocstorecommit.ListOperationLocations(userId, projectId, uint64(i), pathHash)
		if err != nil {
			return nil, err
		}
	}
	if operationLocations == nil {
		return bytes.NewReader([]byte{}), nil
	}

	ops := make(chan *pb.Operation)
	go func() {
		for _, loc := range operationLocations.GetOpLocs() {
			op, err := s.opdatastorecommit.Read(userId, projectId, pathHash, loc.GetOffset(), loc.GetLength())
			if err != nil {
				log.Panic(err)
			}
			ops <- op
		}
		close(ops)
	}()

	result := new(bytes.Buffer)
	resultChunker, err := fastcdc.NewChunker(result, fastcdc.Options{
		AverageSize: 1024 * 64,
		Seed:        84372,
	})
	if err != nil {
		log.Panic(err)
	}
	targetBuffer := bytes.NewBuffer([]byte{})
	err = resultChunker.ApplyDelta(result, bytes.NewReader(targetBuffer.Bytes()), ops)
	if err != nil {
		log.Panic(err)
	}

	return bytes.NewReader(result.Bytes()), nil
}

func (s JamsyncServer) ReadCommittedFile(in *pb.ReadCommittedFileRequest, srv pb.JamsyncAPI_ReadCommittedFileServer) error {
	userId, err := serverauth.ParseIdFromCtx(srv.Context())
	if err != nil {
		return err
	}

	commitId := in.GetCommitId()
	if commitId == 0 {
		maxCommitId, err := s.oplocstorecommit.MaxCommitId(userId, in.GetProjectId())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		commitId = maxCommitId
	}

	sourceBuffer, err := s.regenCommittedFile(userId, in.GetProjectId(), commitId, in.GetPathHash())
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
		err = srv.Send(&pb.ProjectOperation{
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

func (s JamsyncServer) MergeBranch(ctx context.Context, in *pb.MergeBranchRequest) (*pb.MergeBranchResponse, error) {
	userId, err := serverauth.ParseIdFromCtx(ctx)
	if err != nil {
		if in.GetProjectId() != 1 {
			return nil, err
		}
	}

	isFirstCommit := false
	prevCommitId, err := s.oplocstorecommit.MaxCommitId(userId, in.GetProjectId())
	if err != nil && errors.Is(err, os.ErrNotExist) {
		isFirstCommit = true
	} else if err != nil {
		return nil, err
	}

	// Regen every file that has been changed in branch
	changedPathHashes, err := s.opdatastorebranch.GetChangedPathHashes(userId, in.GetProjectId(), in.GetBranchId())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(changedPathHashes) == 0 {
		// NO CHANGES
		return &pb.MergeBranchResponse{CommitId: prevCommitId}, nil
	}

	maxChangeId, err := s.oplocstorebranch.MaxChangeId(userId, in.GetProjectId(), in.GetBranchId())
	if err != nil {
		return nil, err
	}

	pathHashes := make(chan []byte)
	results := make(chan error)

	makeDiff := func() {
		for changedPathHash := range pathHashes {
			sourceReader, err := s.regenBranchFile(userId, in.GetProjectId(), in.GetBranchId(), maxChangeId, changedPathHash)
			if err != nil {
				results <- err
				return
			}

			branchOperationLocations, err := s.ReadCommitChunkHashes(ctx, &pb.ReadCommitChunkHashesRequest{
				ProjectId: in.GetProjectId(),
				CommitId:  prevCommitId,
				PathHash:  []byte(changedPathHash),
			})
			if err != nil {
				results <- err
				return
			}

			sourceChunker, err := fastcdc.NewChunker(sourceReader, fastcdc.Options{
				AverageSize: 1024 * 64,
				Seed:        84372,
			})
			if err != nil {
				results <- err
				return
			}

			opsOut := make(chan *pb.Operation)
			go func() {
				var blockCt, dataCt, bytes int
				defer close(opsOut)
				err := sourceChunker.CreateDelta(branchOperationLocations.GetChunkHashes(), func(op *pb.Operation) error {
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

			pathHashToOpLocs := make(map[string][]*pb.CommitOperationLocations_OperationLocation, 0)
			for op := range opsOut {
				offset, length, err := s.opdatastorecommit.Write(userId, in.GetProjectId(), []byte(changedPathHash), op)
				if err != nil {
					results <- err
					return
				}

				var chunkHash *pb.ChunkHash
				if op.GetType() == pb.Operation_OpData {
					chunkHash = &pb.ChunkHash{
						Offset: op.GetChunk().GetOffset(),
						Length: op.GetChunk().GetLength(),
						Hash:   op.GetChunk().GetHash(),
					}
				} else {
					chunkHash = &pb.ChunkHash{
						Offset: op.GetChunkHash().GetOffset(),
						Length: op.GetChunkHash().GetLength(),
						Hash:   op.GetChunkHash().GetHash(),
					}
				}

				if op.GetType() == pb.Operation_OpBlock {
					opLocs, err := s.oplocstorecommit.ListOperationLocations(userId, in.GetProjectId(), prevCommitId, []byte(changedPathHash))
					if err != nil {
						results <- err
						return
					}
					found := false
					var reusedOffset, reusedLength uint64
					for _, loc := range opLocs.GetOpLocs() {
						if loc.GetChunkHash().GetHash() == op.GetChunkHash().GetHash() {
							found = true
							reusedOffset = loc.GetOffset()
							reusedLength = loc.GetLength()
							break
						}
					}
					if !found {
						log.Fatal("Operation of type block but hash could not be found")
					}
					offset = reusedOffset
					length = reusedLength
				}

				operationLocation := &pb.CommitOperationLocations_OperationLocation{
					Offset:    offset,
					Length:    length,
					ChunkHash: chunkHash,
				}
				pathHashToOpLocs[string(changedPathHash)] = append(pathHashToOpLocs[string(changedPathHash)], operationLocation)
			}

			if isFirstCommit {
				for pathHash, opLocs := range pathHashToOpLocs {
					err = s.oplocstorecommit.InsertOperationLocations(&pb.CommitOperationLocations{
						ProjectId: in.GetProjectId(),
						OwnerId:   userId,
						CommitId:  0,
						PathHash:  []byte(pathHash),
						OpLocs:    opLocs,
					})
					if err != nil {
						results <- err
						return
					}
				}
			} else {
				for pathHash, opLocs := range pathHashToOpLocs {
					err = s.oplocstorecommit.InsertOperationLocations(&pb.CommitOperationLocations{
						ProjectId: in.GetProjectId(),
						OwnerId:   userId,
						CommitId:  prevCommitId + 1,
						PathHash:  []byte(pathHash),
						OpLocs:    opLocs,
					})
					if err != nil {
						results <- err
						return
					}
				}

			}

			results <- nil
		}
	}

	for i := 0; i < 64; i++ {
		go makeDiff()
	}

	go func() {
		for _, c := range changedPathHashes {
			pathHashes <- c
		}
	}()

	completed := 0
	for e := range results {
		if e != nil {
			panic(e)
		}
		completed += 1

		if completed == len(changedPathHashes) {
			close(pathHashes)
			close(results)
		}
	}

	if isFirstCommit {
		return &pb.MergeBranchResponse{
			CommitId: prevCommitId,
		}, nil
	} else {
		return &pb.MergeBranchResponse{
			CommitId: prevCommitId + 1,
		}, nil
	}
}
