package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
)

type JamHubDb struct {
	db *sql.DB
}

func New() (jamhubDB JamHubDb) {
	err := os.MkdirAll("./jamhubdata", os.ModePerm)
	if err != nil {
		panic(err)
	}
	conn, err := sql.Open("sqlite3", "./jamhubdata/jamhub.db")
	if err != nil {
		panic(err)
	}

	sqlStmt := `
	CREATE TABLE IF NOT EXISTS users (username TEXT, user_id TEXT, UNIQUE(username, user_id));
	CREATE TABLE IF NOT EXISTS projects (name TEXT, owner TEXT, UNIQUE(name, owner));
	`
	_, err = conn.Exec(sqlStmt)
	if err != nil {
		panic(err)
	}
	row := conn.QueryRow("SELECT rowid FROM projects WHERE name = ? AND owner = ?", "test", "2")
	if row.Err() != nil {
		panic(row.Err())
	}

	var id uint64
	row.Scan(&id)
	return JamHubDb{conn}
}

type Project struct {
	Name string
	Id   uint64
}

func (j JamHubDb) AddProject(projectName string, owner string) (uint64, error) {
	_, err := j.GetProjectId(projectName, owner)
	if !errors.Is(sql.ErrNoRows, err) {
		return 0, fmt.Errorf("project already exists")
	}

	res, err := j.db.Exec("INSERT INTO projects(name, owner) VALUES(?, ?)", projectName, owner)
	if err != nil {
		return 0, err
	}

	var id int64
	if id, err = res.LastInsertId(); err != nil {
		return 0, err
	}

	return uint64(id), nil
}

func (j JamHubDb) DeleteProject(projectName string, owner string) (uint64, error) {
	_, err := j.GetProjectId(projectName, owner)
	if errors.Is(sql.ErrNoRows, err) {
		return 0, fmt.Errorf("project does not exist")
	}

	res, err := j.db.Exec("DELETE FROM projects WHERE name = ? AND owner = ?", projectName, owner)
	if err != nil {
		return 0, err
	}

	var id int64
	if id, err = res.LastInsertId(); err != nil {
		return 0, err
	}

	return uint64(id), nil
}

func (j JamHubDb) GetProjectOwner(projectId uint64) (string, error) {
	row := j.db.QueryRow("SELECT owner FROM projects WHERE rowid = ?", projectId)
	if row.Err() != nil {
		return "", row.Err()
	}

	var owner string
	err := row.Scan(&owner)
	return owner, err
}

func (j JamHubDb) GetProjectId(projectName string, owner string) (uint64, error) {
	row := j.db.QueryRow("SELECT rowid FROM projects WHERE name = ? AND owner = ?", projectName, owner)
	if row.Err() != nil {
		return 0, row.Err()
	}

	var id uint64
	err := row.Scan(&id)
	return id, err
}

func (j JamHubDb) GetProjectName(id uint64, owner string) (string, error) {
	row := j.db.QueryRow("SELECT name FROM projects WHERE rowid = ? AND owner = ?", id, owner)
	if row.Err() != nil {
		return "", row.Err()
	}

	var name string
	err := row.Scan(&name)
	return name, err
}

func (j JamHubDb) ListUserProjects(owner string) ([]Project, error) {
	rows, err := j.db.Query("SELECT rowid, name FROM projects WHERE owner = ?", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := make([]Project, 0)
	for rows.Next() {
		u := Project{}
		err = rows.Scan(&u.Id, &u.Name)
		if err != nil {
			return nil, err
		}
		data = append(data, u)
	}
	return data, err
}

func (j JamHubDb) CreateUser(username, userId string) error {
	_, err := j.db.Exec("INSERT OR IGNORE INTO users(username, user_id) VALUES (?, ?)", username, userId)
	return err
}

func (j JamHubDb) Username(userId string) (string, error) {
	row := j.db.QueryRow("SELECT username FROM users WHERE user_id = ?", userId)
	if row.Err() != nil {
		return "", row.Err()
	}

	var username string
	err := row.Scan(&username)
	return username, err
}
