package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/schollz/progressbar/v3"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/fastcdc"
	"github.com/zdgeier/jamsync/internal/jamignore"
	"github.com/zdgeier/jamsync/internal/server/file"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func diffHasChanges(diff *pb.FileMetadataDiff) bool {
	for _, diff := range diff.GetDiffs() {
		if diff.Type != pb.FileMetadataDiff_NoOp {
			return true
		}
	}
	return false
}

type PathFile struct {
	path string
	file *pb.File
}

type PathInfo struct {
	path  string
	isDir bool
}

func worker(pathInfos <-chan PathInfo, results chan<- PathFile) {
	for pathInfo := range pathInfos {
		osFile, err := os.Open(pathInfo.path)
		if err != nil {
			fmt.Println("Could not open ", pathInfo.path, ":", err)
			results <- PathFile{}
			continue
		}

		stat, err := osFile.Stat()
		if err != nil {
			panic(err)
		}

		var file *pb.File
		if pathInfo.isDir {
			file = &pb.File{
				ModTime: timestamppb.New(stat.ModTime()),
				Dir:     true,
			}
		} else {
			data, err := os.ReadFile(pathInfo.path)
			if err != nil {
				fmt.Println("Could not read ", pathInfo.path, "(jamsync does not support symlinks)")
				results <- PathFile{}
				continue
			}
			b := xxh3.Hash128(data).Bytes()

			file = &pb.File{
				ModTime: timestamppb.New(stat.ModTime()),
				Dir:     false,
				Hash:    b[:],
			}
		}
		osFile.Close()
		results <- PathFile{pathInfo.path, file}
	}
}

func readLocalFileList() *pb.FileMetadata {
	if os.Args[1] != "sync" {
		fmt.Println("Hashing files")
	}
	var ignorer = &jamignore.JamsyncIgnorer{}
	err := ignorer.ImportPatterns(".gitignore")
	if err != nil {
		panic(err)
	}
	err = ignorer.ImportPatterns(".jamignore")
	if err != nil {
		panic(err)
	}
	var numEntries int64
	i := 0
	if err := filepath.WalkDir(".", func(path string, d fs.DirEntry, _ error) error {
		path = filepath.ToSlash(path)
		if ignorer.Match(path) || path == "." || strings.HasPrefix(path, ".git/") || strings.HasPrefix(path, ".jamsync") {
			return nil
		}
		numEntries += 1
		i += 1
		return nil
	}); err != nil {
		fmt.Println("WARN: could not walk directory tree", err)
	}
	paths := make(chan PathInfo, numEntries)
	results := make(chan PathFile, numEntries)

	i = 0
	for w := 1; w < 4000 && w <= int(numEntries)/10+1; w++ {
		go worker(paths, results)
	}

	go func() {
		if err := filepath.WalkDir(".", func(path string, d fs.DirEntry, _ error) error {
			path = filepath.ToSlash(path)
			if ignorer.Match(path) || path == "." || strings.HasPrefix(path, ".git/") || strings.HasPrefix(path, ".jamsync") {
				return nil
			}
			paths <- PathInfo{path, d.IsDir()}
			i += 1
			return nil
		}); err != nil {
			fmt.Println("WARN: could not walk directory tree", err)
		}
		close(paths)
	}()

	files := make(map[string]*pb.File, numEntries)
	if os.Args[1] != "sync" {
		bar := progressbar.Default(numEntries)
		for i := int64(0); i < numEntries; i++ {
			pathFile := <-results
			if pathFile.path != "" {
				files[pathFile.path] = pathFile.file
			}
			bar.Add(1)
		}
	} else {
		for i := int64(0); i < numEntries; i++ {
			pathFile := <-results
			if pathFile.path != "" {
				files[pathFile.path] = pathFile.file
			}
		}
	}

	return &pb.FileMetadata{
		Files: files,
	}
}

func uploadBranchFiles(ctx context.Context, apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64, paths <-chan string, results chan<- error, numFiles int64) {
	type pathResponse struct {
		chunkHashResponse *pb.ReadBranchChunkHashesResponse
		path              string
	}
	chunkHashResponses := make(chan pathResponse, numFiles)

	numUpload := 500
	numUploadFinished := make(chan bool)
	for i := 0; i < numUpload; i++ {
		go func() {
			writeStream, err := apiClient.WriteBranchOperationsStream(ctx)
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
				if err != nil {
					results <- nil
				}

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
						ProjectId: projectId,
						BranchId:  branchId,
						PathHash:  pathToHash(resp.path),
						Op:        op,
					})
					if err != nil {
						log.Panic(err)
					}
					sent += 1
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
				chunkHashResp, err := apiClient.ReadBranchChunkHashes(ctx, &pb.ReadBranchChunkHashesRequest{
					ProjectId: projectId,
					BranchId:  branchId,
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

func uploadFile(apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64, changeId uint64, filePath string, sourceReader io.Reader) error {
	chunkHashesResp, err := apiClient.ReadBranchChunkHashes(context.TODO(), &pb.ReadBranchChunkHashesRequest{
		ProjectId: projectId,
		BranchId:  branchId,
		ChangeId:  changeId,
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

	writeStream, err := apiClient.WriteBranchOperationsStream(context.TODO())
	if err != nil {
		return err
	}
	for op := range opsOut {
		err = writeStream.Send(&pb.ProjectOperation{
			ProjectId: projectId,
			BranchId:  branchId,
			PathHash:  pathToHash(filePath),
			Op:        op,
		})
		if err != nil {
			log.Panic(err)
		}
	}

	_, err = writeStream.CloseAndRecv()
	return err
}

func pathToHash(path string) []byte {
	h := xxh3.Hash128([]byte(path)).Bytes()
	return h[:]
}

func pushFileListDiffBranch(apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64, prevChangeId uint64, fileMetadata *pb.FileMetadata, fileMetadataDiff *pb.FileMetadataDiff) error {
	ctx := context.Background()

	var numFiles int64
	for _, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetType() != pb.FileMetadataDiff_Delete && !diff.GetFile().GetDir() {
			numFiles += 1
		}
	}

	paths := make(chan string, numFiles)
	results := make(chan error, numFiles)

	go uploadBranchFiles(ctx, apiClient, projectId, branchId, paths, results, numFiles)

	for path, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetType() != pb.FileMetadataDiff_Delete && !diff.GetFile().GetDir() {
			paths <- path
		}
	}
	close(paths)

	fmt.Println("Syncing files")
	bar := progressbar.Default(numFiles)
	for res := range results {
		if res != nil {
			log.Panic(res)
		}
		bar.Add(1)
	}

	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return err
	}
	err = uploadFile(apiClient, projectId, branchId, prevChangeId, ".jamsyncfilelist", bytes.NewReader(metadataBytes))
	if err != nil {
		return err
	}

	return nil
}

func downloadBranchFiles(ctx context.Context, apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64, changeId uint64, paths <-chan string, results chan<- error, numFiles int64) {
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

				readFileClient, err := apiClient.ReadBranchFile(ctx, &pb.ReadBranchFileRequest{
					ProjectId:   projectId,
					BranchId:    branchId,
					ChangeId:    changeId,
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

func downloadCommittedFiles(ctx context.Context, apiClient pb.JamsyncAPIClient, projectId, commitId uint64, paths <-chan string, results chan<- error, numFiles int64) {
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

				readFileClient, err := apiClient.ReadCommittedFile(ctx, &pb.ReadCommittedFileRequest{
					ProjectId:   projectId,
					CommitId:    commitId,
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

func applyFileListDiffCommit(apiClient pb.JamsyncAPIClient, projectId, commitId uint64, fileMetadataDiff *pb.FileMetadataDiff) error {
	ctx := context.Background()
	for path, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetFile().GetDir() {
			err := os.MkdirAll(path, os.ModePerm)
			if err != nil {
				return err
			}
		}
	}
	var numFiles int64
	for _, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetType() != pb.FileMetadataDiff_Delete && !diff.GetFile().GetDir() {
			numFiles += 1
		}
	}

	if numFiles == 0 {
		return nil
	}

	paths := make(chan string, numFiles)
	results := make(chan error, numFiles)

	go downloadCommittedFiles(ctx, apiClient, projectId, commitId, paths, results, numFiles)

	for path, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetType() != pb.FileMetadataDiff_Delete && !diff.GetFile().GetDir() {
			paths <- path
		}
	}
	close(paths)

	if os.Args[1] != "sync" {
		fmt.Println("Syncing files")
		bar := progressbar.Default(numFiles)
		for res := range results {
			if res != nil {
				fmt.Println(res) // Probably should handle this better
			}
			bar.Add(1)
		}
	} else {
		for res := range results {
			if res != nil {
				log.Panic(res)
			}
		}

	}
	return nil
}

func applyFileListDiffBranch(apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64, changeId uint64, fileMetadataDiff *pb.FileMetadataDiff) error {
	ctx := context.Background()
	for path, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetFile().GetDir() {
			err := os.MkdirAll(path, os.ModePerm)
			if err != nil {
				return err
			}
		}
	}
	var numFiles int64
	for _, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetType() != pb.FileMetadataDiff_Delete && !diff.GetFile().GetDir() {
			numFiles += 1
		}
	}

	if numFiles == 0 {
		return nil
	}

	paths := make(chan string, numFiles)
	results := make(chan error, numFiles)

	go downloadBranchFiles(ctx, apiClient, projectId, branchId, changeId, paths, results, numFiles)

	for path, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetType() != pb.FileMetadataDiff_Delete && !diff.GetFile().GetDir() {
			paths <- path
		}
	}
	close(paths)

	if os.Args[1] != "sync" {
		fmt.Println("Syncing files")
		bar := progressbar.Default(numFiles)
		for res := range results {
			if res != nil {
				fmt.Println(res) // Probably should handle this better
			}
			bar.Add(1)
		}
	} else {
		for res := range results {
			if res != nil {
				log.Panic(res)
			}
		}

	}
	return nil
}

func diffRemoteToLocalCommit(apiClient pb.JamsyncAPIClient, projectId uint64, commitId uint64, fileMetadata *pb.FileMetadata) (*pb.FileMetadataDiff, error) {
	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return nil, err
	}
	metadataReader := bytes.NewReader(metadataBytes)
	metadataResult := new(bytes.Buffer)
	err = file.DownloadCommittedFile(apiClient, projectId, commitId, ".jamsyncfilelist", metadataReader, metadataResult)
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

func diffRemoteToLocalBranch(apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64, changeId uint64, fileMetadata *pb.FileMetadata) (*pb.FileMetadataDiff, error) {
	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return nil, err
	}
	metadataReader := bytes.NewReader(metadataBytes)
	metadataResult := new(bytes.Buffer)
	err = file.DownloadBranchFile(apiClient, projectId, branchId, changeId, ".jamsyncfilelist", metadataReader, metadataResult)
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

func diffLocalToRemoteCommit(apiClient pb.JamsyncAPIClient, projectId uint64, commitId uint64, fileMetadata *pb.FileMetadata) (*pb.FileMetadataDiff, error) {
	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return nil, err
	}
	metadataReader := bytes.NewReader(metadataBytes)
	metadataResult := new(bytes.Buffer)
	err = file.DownloadCommittedFile(apiClient, projectId, commitId, ".jamsyncfilelist", metadataReader, metadataResult)
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

func diffLocalToRemoteBranch(apiClient pb.JamsyncAPIClient, projectId uint64, branchId uint64, changeId uint64, fileMetadata *pb.FileMetadata) (*pb.FileMetadataDiff, error) {
	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return nil, err
	}
	metadataReader := bytes.NewReader(metadataBytes)
	metadataResult := new(bytes.Buffer)
	err = file.DownloadBranchFile(apiClient, projectId, branchId, changeId, ".jamsyncfilelist", metadataReader, metadataResult)
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
