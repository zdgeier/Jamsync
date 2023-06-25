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

func (s LocalChangeStore) GetWorkspaceNameById(ownerId string, projectId uint64, workspaceId uint64) (string, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return "", err
	}
	return getWorkspaceNameById(db, workspaceId)
}

func (s LocalChangeStore) GetWorkspaceIdByName(ownerId string, projectId uint64, workspaceName string) (uint64, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return 0, err
	}
	return getWorkspaceIdByName(db, workspaceName)
}

func (s LocalChangeStore) GetWorkspaceBaseCommitId(ownerId string, projectId uint64, workspaceId uint64) (uint64, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return 0, err
	}
	return getWorkspaceBaseCommitId(db, workspaceId)
}

func (s LocalChangeStore) DeleteWorkspace(ownerId string, projectId uint64, workspaceId uint64) error {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return err
	}
	return deleteWorkspace(db, workspaceId)
}

func (s LocalChangeStore) AddWorkspace(ownerId string, projectId uint64, workspaceName string, commitId uint64) (uint64, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return 0, err
	}
	return addWorkspace(db, workspaceName, commitId)
}

func (s LocalChangeStore) ListWorkspaces(ownerId string, projectId uint64) (map[string]uint64, error) {
	db, err := s.getLocalProjectDB(ownerId, projectId)
	if err != nil {
		return nil, err
	}

	return listWorkspaces(db)
}

func (s LocalChangeStore) DeleteProject(projectId uint64, ownerId string) error {
	return os.RemoveAll(fmt.Sprintf("jamhubdata/%s/%d", ownerId, projectId))
}
