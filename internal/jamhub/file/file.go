package file

import (
	"context"
	"io"
	"log"

	"github.com/zdgeier/jamhub/gen/pb"
	"github.com/zdgeier/jamhub/internal/fastcdc"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func DownloadCommittedFile(client pb.JamHubClient, projectId uint64, commitId uint64, filePath string, localReader io.ReadSeeker, localWriter io.Writer) error {
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

	stream, err := client.ReadCommittedFile(context.TODO(), &pb.ReadCommittedFileRequest{
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

func DownloadWorkspaceFile(client pb.JamHubClient, projectId uint64, workspaceId uint64, changeId uint64, filePath string, localReader io.ReadSeeker, localWriter io.Writer) error {
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

	stream, err := client.ReadWorkspaceFile(context.TODO(), &pb.ReadWorkspaceFileRequest{
		ProjectId:   projectId,
		WorkspaceId: workspaceId,
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

func pathToHash(path string) []byte {
	h := xxh3.Hash128([]byte(path)).Bytes()
	return h[:]
}
