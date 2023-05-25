package opdatastorebranch

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/zdgeier/jamsync/gen/pb"
	"google.golang.org/protobuf/proto"
)

type LocalStore struct {
	cache *lru.Cache[string, *os.File]
	mu    sync.Mutex
}

func NewOpDataStoreBranch() *LocalStore {
	cache, err := lru.NewWithEvict(2048, func(path string, file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println(err)
			return
		}
	})
	if err != nil {
		panic(err)
	}
	return &LocalStore{
		cache: cache,
	}
}

func (s *LocalStore) filePath(ownerId string, projectId, branchId uint64, pathHash []byte) string {
	return fmt.Sprintf("jb/%s/%d/opdatabranch/%d/%02X/%02X.locs", ownerId, projectId, branchId, pathHash[:1], pathHash)
}

func (s *LocalStore) fileDir(ownerId string, projectId, branchId uint64, pathHash []byte) string {
	return fmt.Sprintf("jb/%s/%d/opdatabranch/%d/%02X", ownerId, projectId, branchId, pathHash[:1])
}

func (s *LocalStore) Read(ownerId string, projectId, branchId uint64, pathHash []byte, offset uint64, length uint64) (*pb.Operation, error) {
	filePath := s.filePath(ownerId, projectId, branchId, pathHash)
	var (
		currFile *os.File
		err      error
	)
	if s.cache.Contains(filePath) {
		currFile, _ = s.cache.Get(filePath)
	} else {
		currFile, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return nil, err
		}
		s.cache.Add(filePath, currFile)
	}
	s.mu.Lock()
	b := make([]byte, length)
	_, err = currFile.ReadAt(b, int64(offset))
	if err != nil {
		return nil, err
	}
	s.mu.Unlock()

	op := new(pb.Operation)
	err = proto.Unmarshal(b, op)
	if err != nil {
		log.Panic(err)
	}
	return op, nil
}

func (s *LocalStore) Write(ownerId string, projectId, branchId uint64, pathHash []byte, op *pb.Operation) (offset uint64, length uint64, err error) {
	err = os.MkdirAll(s.fileDir(ownerId, projectId, branchId, pathHash), os.ModePerm)
	if err != nil {
		return 0, 0, err
	}

	filePath := s.filePath(ownerId, projectId, branchId, pathHash)
	var currFile *os.File
	if s.cache.Contains(filePath) {
		currFile, _ = s.cache.Get(filePath)
	} else {
		currFile, err = os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return 0, 0, err
		}
		s.cache.Add(filePath, currFile)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := currFile.Stat()
	if err != nil {
		return 0, 0, err
	}
	data, err := proto.Marshal(op)
	if err != nil {
		return 0, 0, err
	}
	writtenBytes, err := currFile.Write(data)
	if err != nil {
		return 0, 0, err
	}
	return uint64(info.Size()), uint64(writtenBytes), nil
}

func (s *LocalStore) GetChangedPathHashes(ownerId string, projectId uint64, branchId uint64) [][]byte {
	projectDataDir := fmt.Sprintf("jb/%s/%d/opdatabranch/%d", ownerId, projectId, branchId)
	dirs, err := ioutil.ReadDir(projectDataDir)
	if err != nil {
		log.Panic(err)
	}

	pathHashes := make([][]byte, 0)
	for _, dir := range dirs {
		files, err := ioutil.ReadDir(filepath.Join(projectDataDir, dir.Name()))
		if err != nil {
			log.Panic(err)
		}

		for _, file := range files {
			data, err := hex.DecodeString(strings.TrimSuffix(file.Name(), ".locs"))
			if err != nil {
				panic(err)
			}
			pathHashes = append(pathHashes, data)
		}
	}

	return pathHashes
}

func (s *LocalStore) DeleteProject(ownerId string, projectId uint64) error {
	return os.RemoveAll(fmt.Sprintf("jb/%s/%d/opdatabranch", ownerId, projectId))
}

func (s *LocalStore) DeleteBranch(ownerId string, projectId uint64, branchId uint64) error {
	dirs, err := ioutil.ReadDir(fmt.Sprintf("jb/%s/%d/opdatabranch/%d", ownerId, projectId, branchId))
	if err != nil {
		log.Panic(err)
	}

	for _, dir := range dirs {
		err := os.RemoveAll(fmt.Sprintf("jb/%s/%d/opdatabranch/%d/%s", ownerId, projectId, branchId, dir.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}
