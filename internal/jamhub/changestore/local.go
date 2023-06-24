package changestore

import (
	"database/sql"
	"fmt"
	"os"
)

type LocalChangeStore struct {
	dbs map[uint64]*sql.DB
}

func NewLocalChangeStore() LocalChangeStore {
	return LocalChangeStore{
		dbs: make(map[uint64]*sql.DB, 0),
	}
}

func (s LocalChangeStore) getLocalProjectDB(ownerId string, projectId uint64) (*sql.DB, error) {
	if db, ok := s.dbs[projectId]; ok {
		return db, nil
	}

	dir := fmt.Sprintf("jamhubdata/%s/%d", ownerId, projectId)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, err
	}

	localDB, err := sql.Open("sqlite3", dir+"/jamhubproject.db")
	if err != nil {
		return nil, err
	}
	err = setup(localDB)
	if err != nil {
		return nil, err
	}

	s.dbs[projectId] = localDB
	return localDB, nil
}

func (s LocalChangeStore) GetBranchNameById(ownerId string, projectId uint64, branchId uint64) (string, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return "", err
	}
	return getBranchNameById(db, branchId)
}

func (s LocalChangeStore) GetBranchIdByName(ownerId string, projectId uint64, branchName string) (uint64, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return 0, err
	}
	return getBranchIdByName(db, branchName)
}

func (s LocalChangeStore) GetBranchBaseCommitId(ownerId string, projectId uint64, branchId uint64) (uint64, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return 0, err
	}
	return getBranchBaseCommitId(db, branchId)
}

func (s LocalChangeStore) DeleteBranch(ownerId string, projectId uint64, branchId uint64) error {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return err
	}
	return deleteBranch(db, branchId)
}

func (s LocalChangeStore) AddBranch(ownerId string, projectId uint64, branchName string, commitId uint64) (uint64, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return 0, err
	}
	return addBranch(db, branchName, commitId)
}

func (s LocalChangeStore) ListBranches(ownerId string, projectId uint64) (map[string]uint64, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return nil, err
	}

	return listBranches(db)
}

func (s LocalChangeStore) DeleteProject(projectId uint64, ownerId string) error {
	return os.RemoveAll(fmt.Sprintf("jamhubdata/%s/%d", ownerId, projectId))
}
