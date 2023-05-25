package opdatastorecommit

import (
	"fmt"
	"log"
	"os"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/zdgeier/jamsync/gen/pb"
	"google.golang.org/protobuf/proto"
)

type LocalStore struct {
	cache *lru.Cache[string, *os.File]
	mu    sync.Mutex
}

func NewOpDataStoreCommit() *LocalStore {
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

func (s *LocalStore) filePath(ownerId string, projectId uint64, pathHash []byte) string {
	return fmt.Sprintf("jb/%s/%d/opdatacommit/%02X/%02X.locs", ownerId, projectId, pathHash[:1], pathHash)
}

func (s *LocalStore) fileDir(ownerId string, projectId uint64, pathHash []byte) string {
	return fmt.Sprintf("jb/%s/%d/opdatacommit/%02X", ownerId, projectId, pathHash[:1])
}

func (s *LocalStore) Read(ownerId string, projectId uint64, pathHash []byte, offset uint64, length uint64) (*pb.Operation, error) {
	filePath := s.filePath(ownerId, projectId, pathHash)
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

func (s *LocalStore) Write(ownerId string, projectId uint64, pathHash []byte, op *pb.Operation) (offset uint64, length uint64, err error) {
	err = os.MkdirAll(s.fileDir(ownerId, projectId, pathHash), os.ModePerm)
	if err != nil {
		return 0, 0, err
	}
	filePath := s.filePath(ownerId, projectId, pathHash)
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

func (s *LocalStore) DeleteProject(ownerId string, projectId uint64) error {
	return os.RemoveAll(fmt.Sprintf("jb/%s/%d/opdatacommit", ownerId, projectId))
}
