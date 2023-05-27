package file

import (
	"bytes"
	"context"
	"io"
	"log"
	"path/filepath"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/fastcdc"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func UploadBranchFile(ctx context.Context, apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64, filePath string, sourceReader io.Reader) error {
	chunkHashesResp, err := apiClient.ReadBranchChunkHashes(ctx, &pb.ReadBranchChunkHashesRequest{
		ProjectId: projectId,
		BranchId:  branchId,
		PathHash:  pathToHash(filePath),
		ModTime:   timestamppb.Now(),
	})
	if err != nil {
		return err
	}

	sourceChunker, err := fastcdc.NewChunker(sourceReader, fastcdc.Options{
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
		err := sourceChunker.CreateDelta(chunkHashesResp.GetChunkHashes(), func(op *pb.Operation) error {
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

	writeStream, err := c.api.WriteBranchOperationsStream(ctx)
	if err != nil {
		return err
	}
	sent := 0
	for op := range opsOut {
		err = writeStream.Send(&pb.ProjectOperation{
			ProjectId: client.ProjectId,
			BranchId:  client.BranchId,
			PathHash:  pathToHash(filePath),
			Op:        op,
		})
		if err != nil {
			log.Panic(err)
		}
		sent += 1
	}
	// We have to send a tombstone if we have not generated any ops (empty file)
	if sent == 0 {
		err = writeStream.Send(&pb.ProjectOperation{
			ProjectId: client.ProjectId,
			BranchId:  client.BranchId,
			PathHash:  pathToHash(filePath),
			Op: &pb.Operation{
				Type:  pb.Operation_OpData,
				Chunk: &pb.Chunk{},
			},
		})
		if err != nil {
			log.Panic(err)
		}
	}
	_, err = writeStream.CloseAndRecv()
	return err
}

func DownloadCommittedFile(ctx context.Context, client pb.JamsyncAPIClient, projectId uint64, commitId uint64, filePath string, localReader io.ReadSeeker, localWriter io.Writer) error {
	sig := make([]*pb.ChunkHash, 0)
	localChunker, err := fastcdc.NewChunker(localReader, fastcdc.Options{
		AverageSize: 1024 * 64,
		Seed:        84372,
	})
	if err != nil {
		return err
	}

	err = localChunker.CreateSignature(func(ch *pb.ChunkHash) error {
		sig = append(sig, ch)
		return nil
	})
	if err != nil {
		return err
	}

	stream, err := client.ReadCommittedFile(ctx, &pb.ReadCommittedFileRequest{
		ProjectId:   projectId,
		CommitId:    commitId,
		PathHash:    pathToHash(filePath),
		ModTime:     timestamppb.Now(),
		ChunkHashes: sig,
	})
	if err != nil {
		return err
	}

	numOps := 0
	ops := make(chan *pb.Operation)
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Println(err)
				return
			}
			ops <- in.GetOp()
			numOps += 1
		}
		close(ops)
	}()

	localReader.Seek(0, 0)
	err = localChunker.ApplyDelta(localWriter, localReader, ops)
	if err != nil {
		return err
	}

	return err
}

func DownloadBranchFile(client pb.JamsyncAPIClient, projectId uint64, branchId uint64, changeId uint64, filePath string, localReader io.ReadSeeker, localWriter io.Writer) error {
	sig := make([]*pb.ChunkHash, 0)
	localChunker, err := fastcdc.NewChunker(localReader, fastcdc.Options{
		AverageSize: 1024 * 64,
		Seed:        84372,
	})
	if err != nil {
		return err
	}

	err = localChunker.CreateSignature(func(ch *pb.ChunkHash) error {
		sig = append(sig, ch)
		return nil
	})
	if err != nil {
		return err
	}

	stream, err := client.ReadBranchFile(context.TODO(), &pb.ReadBranchFileRequest{
		ProjectId:   projectId,
		BranchId:    branchId,
		ChangeId:    changeId,
		PathHash:    pathToHash(filePath),
		ModTime:     timestamppb.Now(),
		ChunkHashes: sig,
	})
	if err != nil {
		return err
	}

	ops := make(chan *pb.Operation)
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Println(err)
				return
			}
			ops <- in.GetOp()
		}
		close(ops)
	}()

	localReader.Seek(0, 0)
	err = localChunker.ApplyDelta(localWriter, localReader, ops)
	if err != nil {
		return err
	}

	return err
}

func BrowseProject(path string) (*pb.BrowseProjectResponse, error) {
	ctx := context.Background()
	metadataResult := new(bytes.Buffer)
	err := c.DownloadFile(ctx, ".jamsyncfilelist", bytes.NewReader([]byte{}), metadataResult)
	if err != nil {
		return nil, err
	}
	fileMetadata := &pb.FileMetadata{}
	err = proto.Unmarshal(metadataResult.Bytes(), fileMetadata)
	if err != nil {
		return nil, err
	}

	directoryNames := make([]string, 0, len(fileMetadata.GetFiles()))
	fileNames := make([]string, 0, len(fileMetadata.GetFiles()))
	requestPath := filepath.Clean(path)
	for path, file := range fileMetadata.GetFiles() {
		pathDir := filepath.Dir(path)
		if (path == "" && pathDir == ".") || pathDir == requestPath {
			if file.GetDir() {
				directoryNames = append(directoryNames, filepath.Base(path))
			} else {
				fileNames = append(fileNames, filepath.Base(path))
			}
		}
	}

	return &pb.BrowseProjectResponse{
		Directories: directoryNames,
		Files:       fileNames,
	}, err
}

func pathToHash(path string) []byte {
	h := xxh3.Hash128([]byte(path)).Bytes()
	return h[:]
}
