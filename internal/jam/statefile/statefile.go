package statefile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type WorkspaceInfo struct {
	WorkspaceId uint64 `json:"workspaceid"`
	ChangeId    uint64 `json:"changeid"`
}

type CommitInfo struct {
	CommitId uint64 `json:"commitid"`
}

type StateFile struct {
	ProjectId     uint64 `json:"projectid"`
	WorkspaceInfo *WorkspaceInfo
	CommitInfo    *CommitInfo
}

func (s StateFile) Save() error {
	f, err := os.Create(".jamhub")
	if err != nil {
		return err
	}
	defer f.Close()

	configBytes, err := json.Marshal(s)
	if err != nil {
		return err
	}

	_, err = f.Write(configBytes)
	return err
}

func Find() (StateFile, error) {
	relCurrPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	currentPath, err := filepath.Abs(relCurrPath)
	if err != nil {
		panic(err)
	}
	filePath, err := filepath.Abs(fmt.Sprintf("%v/%v", currentPath, ".jamhub"))
	if err != nil {
		panic(err)
	}
	if configBytes, err := os.ReadFile(filePath); err == nil {
		var stateFile StateFile
		err = json.Unmarshal(configBytes, &stateFile)
		if err != nil {
			panic(err)
		}
		return stateFile, nil
	}
	return StateFile{}, errors.New("could not find .jamhub file")
}
