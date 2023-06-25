package changestore

import (
	"database/sql"
	"errors"
)

func setup(db *sql.DB) error {
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS workspaces (name TEXT, baseCommitId INTEGER, deleted INTEGER, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP);
	`
	_, err := db.Exec(sqlStmt)
	return err
}

func deleteWorkspace(db *sql.DB, workspaceId uint64) error {
	_, err := db.Exec("UPDATE workspaces SET deleted = 1 WHERE rowid = ?", workspaceId)
	if err != nil {
		return err
	}

	return err
}

func getWorkspaceIdByName(db *sql.DB, workspaceName string) (uint64, error) {
	row := db.QueryRow("SELECT rowid FROM workspaces WHERE name = ?", workspaceName)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var workspaceId uint64
	err := row.Scan(&workspaceId)
	if errors.Is(sql.ErrNoRows, err) {
		return 0, nil
	}
	return workspaceId, err
}

func getWorkspaceBaseCommitId(db *sql.DB, workspaceId uint64) (uint64, error) {
	row := db.QueryRow("SELECT baseCommitId FROM workspaces WHERE rowid = ?", workspaceId)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var commitId uint64
	err := row.Scan(&commitId)
	if errors.Is(sql.ErrNoRows, err) {
		return 0, nil
	}
	return commitId, err
}

func getWorkspaceNameById(db *sql.DB, workspaceId uint64) (string, error) {
	row := db.QueryRow("SELECT name FROM workspaces WHERE rowid = ?", workspaceId)
	if row.Err() != nil {
		return "", row.Err()
	}

	var name string
	err := row.Scan(&name)
	if errors.Is(sql.ErrNoRows, err) {
		return "", nil
	}
	return name, err
}

func addWorkspace(db *sql.DB, workspaceName string, baseCommitId uint64) (uint64, error) {
	res, err := db.Exec("INSERT INTO workspaces(name, baseCommitId, deleted) VALUES(?, ?, 0)", workspaceName, baseCommitId)
	if err != nil {
		return 0, err
	}

	rowId, err := res.LastInsertId()
	if err != nil {
		return uint64(rowId), err
	}

	return uint64(rowId), err
}

func listWorkspaces(db *sql.DB) (map[string]uint64, error) {
	rows, err := db.Query("SELECT rowid, name FROM workspaces WHERE deleted = 0")
	if err != nil {
		return nil, err
	}

	data := make(map[string]uint64, 0)
	for rows.Next() {
		var name string
		var id uint64
		err = rows.Scan(&id, &name)
		if err != nil {
			return nil, err
		}
		data[name] = id
	}

	return data, err
}
