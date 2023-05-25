package changestore

import (
	"database/sql"
	"errors"
)

func setup(db *sql.DB) error {
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS branches (name TEXT, commitId INTEGER, deleted INTEGER, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP);
	`
	_, err := db.Exec(sqlStmt)
	return err
}

func deleteBranch(db *sql.DB, branchId uint64) error {
	_, err := db.Exec("UPDATE branches SET deleted = 1 WHERE rowid = ?", branchId)
	if err != nil {
		return err
	}

	return err
}

func getBranchByName(db *sql.DB, branchName string) (uint64, uint64, error) {
	row := db.QueryRow("SELECT rowid, commitId FROM branches WHERE name = ?", branchName)
	if row.Err() != nil {
		return 0, 0, row.Err()
	}

	var commitId uint64
	var branchId uint64
	err := row.Scan(&branchId, &commitId)
	if errors.Is(sql.ErrNoRows, err) {
		return 0, 0, nil
	}
	return branchId, commitId, err
}

func getBranch(db *sql.DB, branchId uint64) (string, uint64, error) {
	row := db.QueryRow("SELECT name, commitId FROM branches WHERE rowid = ?", branchId)
	if row.Err() != nil {
		return "", 0, row.Err()
	}

	var commitId uint64
	var name string
	err := row.Scan(&name, &commitId)
	if errors.Is(sql.ErrNoRows, err) {
		return "", 0, nil
	}
	return name, commitId, err
}

func addBranch(db *sql.DB, branchName string, commitId uint64) (uint64, error) {
	res, err := db.Exec("INSERT INTO branches(name, commitId) VALUES(?, ?)", branchName, commitId)
	if err != nil {
		return 0, err
	}

	rowId, err := res.LastInsertId()
	if err != nil {
		return uint64(rowId), err
	}

	return uint64(rowId), err
}

func listBranches(db *sql.DB) (map[string]uint64, error) {
	rows, err := db.Query("SELECT rowid, name FROM branches")
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
