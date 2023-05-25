package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/fastcdc"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	api       pb.JamsyncAPIClient
	projectId uint64
	branchId  uint64
}

func NewClient(apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64) *Client {
	return &Client{
		api:       apiClient,
		projectId: projectId,
		branchId:  branchId,
	}
}

func (c *Client) UploadFiles(ctx context.Context, paths <-chan string, results chan<- error, numFiles int64) {
	type pathResponse struct {
		chunkHashResponse *pb.ReadBranchChunkHashesResponse
		path              string
	}
	chunkHashResponses := make(chan pathResponse, numFiles)

	numUpload := 500
	numUploadFinished := make(chan bool)
	for i := 0; i < numUpload; i++ {
		go func() {
			writeStream, err := c.api.WriteBranchOperationsStream(ctx)
			if err != nil {
				log.Panic(err)
			}
			for resp := range chunkHashResponses {
				file, err := os.OpenFile(resp.path, os.O_RDONLY, 0755)
				if err != nil {
					results <- nil
				}
				sourceChunker, err := fastcdc.NewChunker(file, fastcdc.Options{
					AverageSize: 1024 * 64,
					Seed:        84372,
				})

				opsOut := make(chan *pb.Operation)
				go func() {
					var blockCt, dataCt, bytes int
					defer close(opsOut)
					err := sourceChunker.CreateDelta(resp.chunkHashResponse.GetChunkHashes(), func(op *pb.Operation) error {
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
						log.Panic(err)
					}
				}()
				sent := 0
				for op := range opsOut {
					err = writeStream.Send(&pb.ProjectOperation{
						ProjectId: c.projectId,
						BranchId:  c.branchId,
						PathHash:  pathToHash(resp.path),
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
						ProjectId: c.projectId,
						BranchId:  c.branchId,
						PathHash:  pathToHash(resp.path),
						Op: &pb.Operation{
							Type:  pb.Operation_OpData,
							Chunk: &pb.Chunk{},
						},
					})
					if err != nil {
						log.Panic(err)
					}
				}
				results <- file.Close()
			}
			_, err = writeStream.CloseAndRecv()
			if err != nil {
				log.Panic(err)
			}

			numUploadFinished <- true
		}()
	}

	done := make(chan bool, 1)
	go func() {
		for i := 0; i < numUpload; i++ {
			<-numUploadFinished
		}
		close(results)
		done <- true
	}()

	numHashDownload := 100
	numHashDownloadFinished := make(chan bool)
	for i := 0; i < numHashDownload; i++ {
		go func() {
			for path := range paths {
				chunkHashResp, err := c.api.ReadBranchChunkHashes(ctx, &pb.ReadBranchChunkHashesRequest{
					ProjectId: c.projectId,
					BranchId:  c.branchId,
					PathHash:  pathToHash(path),
					ModTime:   timestamppb.Now(),
				})
				if err != nil {
					results <- err
					return
				}
				chunkHashResponses <- pathResponse{chunkHashResp, path}
			}
			numHashDownloadFinished <- true
		}()
	}
	for i := 0; i < numHashDownload; i++ {
		<-numHashDownloadFinished
	}
	close(chunkHashResponses)
	<-done
}

func (c *Client) DownloadFiles(ctx context.Context, paths <-chan string, results chan<- error, numFiles int64) {
	numUpload := 100
	numUploadFinished := make(chan bool)
	for i := 0; i < numUpload; i++ {
		go func() {
			for path := range paths {
				currFile, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0755)
				if err != nil {
					fmt.Println(err)
					results <- nil
					continue
				}

				targetChunker, err := fastcdc.NewChunker(currFile, fastcdc.Options{
					AverageSize: 1024 * 64,
					Seed:        84372,
				})
				if err != nil {
					results <- err
					continue
				}

				sig := make([]*pb.ChunkHash, 0)
				err = targetChunker.CreateSignature(func(ch *pb.ChunkHash) error {
					sig = append(sig, ch)
					return nil
				})
				if err != nil {
					results <- err
					continue
				}

				readFileClient, err := c.api.ReadBranchFile(ctx, &pb.ReadBranchFileRequest{
					ProjectId:   c.projectId,
					BranchId:    c.branchId,
					PathHash:    pathToHash(path),
					ModTime:     timestamppb.Now(),
					ChunkHashes: sig,
				})
				if err != nil {
					results <- err
					continue
				}
				numOps := 0
				ops := make(chan *pb.Operation)
				go func() {
					for {
						in, err := readFileClient.Recv()
						if err == io.EOF {
							break
						}
						if err != nil {
							log.Println(err)
							return
						}
						ops <- in.Op
						numOps += 1
					}
					close(ops)
				}()
				tempFilePath := path + ".jamtemp"
				tempFile, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_CREATE, 0755)
				if err != nil {
					results <- err
					continue
				}

				currFile.Seek(0, 0)
				err = targetChunker.ApplyDelta(tempFile, currFile, ops)
				if err != nil {
					results <- err
					continue
				}
				err = currFile.Close()
				if err != nil {
					results <- err
					continue
				}
				err = tempFile.Close()
				if err != nil {
					results <- err
					continue
				}

				err = os.Rename(tempFilePath, path)
				if err != nil {
					fmt.Println(err)
				}

				results <- nil
			}
			numUploadFinished <- true
		}()
	}

	done := make(chan bool, 1)
	go func() {
		for i := 0; i < numUpload; i++ {
			<-numUploadFinished
		}
		close(results)
		done <- true
	}()
	<-done
}

func (c *Client) UploadFile(ctx context.Context, filePath string, sourceReader io.Reader) error {
	chunkHashesResp, err := c.api.ReadBranchChunkHashes(ctx, &pb.ReadBranchChunkHashesRequest{
		ProjectId: c.projectId,
		BranchId:  c.branchId,
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
			ProjectId: c.projectId,
			BranchId:  c.branchId,
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
			ProjectId: c.projectId,
			BranchId:  c.branchId,
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

func (c *Client) UpdateFile(ctx context.Context, path string, data []byte) error {
	hSum := xxh3.Hash128(data).Bytes()

	newFile := &pb.File{
		ModTime: timestamppb.Now(),
		Dir:     false,
		Hash:    hSum[:],
	}

	metadataReader := bytes.NewReader([]byte{})
	metadataResult := new(bytes.Buffer)
	err := c.DownloadFile(ctx, ".jamsyncfilelist", metadataReader, metadataResult)
	if err != nil {
		return err
	}

	remoteFileMetadata := &pb.FileMetadata{}
	err = proto.Unmarshal(metadataResult.Bytes(), remoteFileMetadata)
	if err != nil {
		return err
	}

	remoteFileMetadata.GetFiles()[path] = newFile

	err = c.UploadFile(ctx, path, bytes.NewReader(data))
	if err != nil {
		return err
	}

	metadataBytes, err := proto.Marshal(remoteFileMetadata)
	if err != nil {
		return err
	}

	log.Println("uploading file list")
	newMetadataReader := bytes.NewReader(metadataBytes)
	err = c.UploadFile(ctx, ".jamsyncfilelist", newMetadataReader)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) DiffLocalToRemote(ctx context.Context, fileMetadata *pb.FileMetadata) (*pb.FileMetadataDiff, error) {
	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return nil, err
	}
	metadataReader := bytes.NewReader(metadataBytes)
	metadataResult := new(bytes.Buffer)
	err = c.DownloadFile(ctx, ".jamsyncfilelist", metadataReader, metadataResult)
	if err != nil {
		return nil, err
	}

	remoteFileMetadata := &pb.FileMetadata{}
	err = proto.Unmarshal(metadataResult.Bytes(), remoteFileMetadata)
	if err != nil {
		return nil, err
	}

	fileMetadataDiff := make(map[string]*pb.FileMetadataDiff_FileDiff, len(remoteFileMetadata.GetFiles()))
	for remoteFilePath := range remoteFileMetadata.GetFiles() {
		fileMetadataDiff[remoteFilePath] = &pb.FileMetadataDiff_FileDiff{
			Type: pb.FileMetadataDiff_Delete,
		}
	}

	for filePath, file := range fileMetadata.GetFiles() {
		var diffFile *pb.File
		diffType := pb.FileMetadataDiff_Delete
		remoteFile, found := remoteFileMetadata.GetFiles()[filePath]
		if found && proto.Equal(file, remoteFile) {
			diffType = pb.FileMetadataDiff_NoOp
		} else if found {
			diffFile = file
			diffType = pb.FileMetadataDiff_Update
		} else {
			diffFile = file
			diffType = pb.FileMetadataDiff_Create
		}

		fileMetadataDiff[filePath] = &pb.FileMetadataDiff_FileDiff{
			Type: diffType,
			File: diffFile,
		}
	}

	return &pb.FileMetadataDiff{
		Diffs: fileMetadataDiff,
	}, err
}

func (c *Client) DiffRemoteToLocal(ctx context.Context, fileMetadata *pb.FileMetadata) (*pb.FileMetadataDiff, error) {
	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return nil, err
	}
	metadataReader := bytes.NewReader(metadataBytes)
	metadataResult := new(bytes.Buffer)
	err = c.DownloadFile(ctx, ".jamsyncfilelist", metadataReader, metadataResult)
	if err != nil {
		return nil, err
	}

	remoteFileMetadata := &pb.FileMetadata{}
	err = proto.Unmarshal(metadataResult.Bytes(), remoteFileMetadata)
	if err != nil {
		return nil, err
	}

	fileMetadataDiff := make(map[string]*pb.FileMetadataDiff_FileDiff, len(fileMetadata.GetFiles()))
	for filePath := range fileMetadata.GetFiles() {
		fileMetadataDiff[filePath] = &pb.FileMetadataDiff_FileDiff{
			Type: pb.FileMetadataDiff_Delete,
		}
	}

	for filePath, file := range remoteFileMetadata.GetFiles() {
		var diffFile *pb.File
		diffType := pb.FileMetadataDiff_Delete
		remoteFile, found := fileMetadata.GetFiles()[filePath]
		if found && proto.Equal(file, remoteFile) {
			diffType = pb.FileMetadataDiff_NoOp
		} else if found {
			diffFile = file
			diffType = pb.FileMetadataDiff_Update
		} else {
			diffFile = file
			diffType = pb.FileMetadataDiff_Create
		}

		fileMetadataDiff[filePath] = &pb.FileMetadataDiff_FileDiff{
			Type: diffType,
			File: diffFile,
		}
	}

	return &pb.FileMetadataDiff{
		Diffs: fileMetadataDiff,
	}, err
}

func (c *Client) DownloadFile(ctx context.Context, filePath string, localReader io.ReadSeeker, localWriter io.Writer) error {
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

	var client pb.JamsyncAPI_ReadBranchFileClient
	if c.branchId == 0 {
		client, err = c.api.ReadCommittedFile(ctx, &pb.ReadCommittedFileRequest{
			ProjectId:   c.projectId,
			PathHash:    pathToHash(filePath),
			ModTime:     timestamppb.Now(),
			ChunkHashes: sig,
		})
		if err != nil {
			return err
		}
	} else {
		client, err = c.api.ReadBranchFile(ctx, &pb.ReadBranchFileRequest{
			ProjectId:   c.projectId,
			BranchId:    c.branchId,
			PathHash:    pathToHash(filePath),
			ModTime:     timestamppb.Now(),
			ChunkHashes: sig,
		})
		if err != nil {
			return err
		}

	}

	numOps := 0
	ops := make(chan *pb.Operation)
	go func() {
		for {
			in, err := client.Recv()
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

func (c *Client) ProjectConfig() *pb.ProjectConfig {
	return &pb.ProjectConfig{
		ProjectId: c.projectId,
		BranchId:  c.branchId,
	}
}

func (c *Client) BrowseProject(path string) (*pb.BrowseProjectResponse, error) {
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
