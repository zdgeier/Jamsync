package changestore

import (
	"database/sql"
	"errors"
)

func setup(db *sql.DB) error {
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS branches (name TEXT, baseCommitId INTEGER, deleted INTEGER, timestamp DATETIME DEFAULT CURRENT_TIMESTAMP);
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

func getBranchIdByName(db *sql.DB, branchName string) (uint64, error) {
	row := db.QueryRow("SELECT rowid, FROM branches WHERE name = ?", branchName)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var branchId uint64
	err := row.Scan(&branchId)
	if errors.Is(sql.ErrNoRows, err) {
		return 0, nil
	}
	return branchId, err
}

func getBranchBaseCommitId(db *sql.DB, branchId uint64) (uint64, error) {
	row := db.QueryRow("SELECT baseCommitId FROM branches WHERE rowid = ?", branchId)
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

func getBranchNameById(db *sql.DB, branchId uint64) (string, error) {
	row := db.QueryRow("SELECT name FROM branches WHERE rowid = ?", branchId)
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

func addBranch(db *sql.DB, branchName string, baseCommitId uint64) (uint64, error) {
	res, err := db.Exec("INSERT INTO branches(name, baseCommitId) VALUES(?, ?)", branchName, baseCommitId)
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
