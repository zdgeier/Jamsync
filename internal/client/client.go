package client

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/schollz/progressbar/v3"
	"github.com/zdgeier/jamsync/gen/pb"
	"github.com/zdgeier/jamsync/internal/jamignore"
	serverclient "github.com/zdgeier/jamsync/internal/server/client"
	"github.com/zdgeier/jamsync/internal/server/clientauth"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func loginAuth() error {
	token, err := clientauth.AuthorizeUser()
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		log.Panic(err)
	}
	err = os.WriteFile(authPath(home), []byte(token), fs.ModePerm)
	if err != nil {
		log.Panic(err)
	}
	return nil
}

func findJamsyncConfig() (*pb.ProjectConfig, string) {
	relCurrPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	currentPath, err := filepath.Abs(relCurrPath)
	if err != nil {
		panic(err)
	}
	filePath, err := filepath.Abs(fmt.Sprintf("%v/%v", currentPath, ".jamsync"))
	if err != nil {
		panic(err)
	}
	if configBytes, err := os.ReadFile(filePath); err == nil {
		config := &pb.ProjectConfig{}
		err = proto.Unmarshal(configBytes, config)
		if err != nil {
			panic(err)
		}
		return config, filePath
	}
	return nil, ""
}

func authPath(home string) string {
	return path.Join(home, ".jamsyncauth")
}

func diffHasChanges(diff *pb.FileMetadataDiff) bool {
	for _, diff := range diff.GetDiffs() {
		if diff.Type != pb.FileMetadataDiff_NoOp {
			return true
		}
	}
	return false
}

func writeJamsyncFile(config *pb.ProjectConfig) error {
	f, err := os.Create(".jamsync")
	if err != nil {
		return err
	}
	defer f.Close()

	configBytes, err := proto.Marshal(config)
	if err != nil {
		return err
	}
	_, err = f.Write(configBytes)
	return err
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

func pushFileListDiff(fileMetadata *pb.FileMetadata, fileMetadataDiff *pb.FileMetadataDiff, client *serverclient.Client) error {
	ctx := context.Background()

	var numFiles int64
	for _, diff := range fileMetadataDiff.GetDiffs() {
		if diff.GetType() != pb.FileMetadataDiff_NoOp && diff.GetType() != pb.FileMetadataDiff_Delete && !diff.GetFile().GetDir() {
			numFiles += 1
		}
	}

	paths := make(chan string, numFiles)
	results := make(chan error, numFiles)

	go client.UploadFiles(ctx, paths, results, numFiles)

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
				log.Panic(res)
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

	metadataBytes, err := proto.Marshal(fileMetadata)
	if err != nil {
		return err
	}
	err = client.UploadFile(ctx, ".jamsyncfilelist", bytes.NewReader(metadataBytes))
	if err != nil {
		return err
	}

	return nil
}

func applyFileListDiff(fileMetadataDiff *pb.FileMetadataDiff, client *serverclient.Client) error {
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

	go client.DownloadFiles(ctx, paths, results, numFiles)

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
