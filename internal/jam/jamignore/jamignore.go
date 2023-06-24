package jamignore

import (
	"os"
	"strings"

	"github.com/gobwas/glob"
)

type JamHubIgnorer struct {
	globs []glob.Glob
}

func NewIgnorer() *JamHubIgnorer {
	return &JamHubIgnorer{
		globs: make([]glob.Glob, 0),
	}
}

func (j *JamHubIgnorer) ImportPatterns(filepath string) error {
	file, err := os.ReadFile(filepath)
	if err != nil {
		return nil
	}

	patterns := strings.Split(string(file), "\n")
	newGlobs := make([]glob.Glob, 0, len(patterns))

	for _, line := range patterns {
		if strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0 {
			continue
		}

		// TODO: Hacking this together for now. This should be made properly.
		line = strings.ReplaceAll(line, "/", "*")

		pat, _, _ := strings.Cut(line, "#")

		lineGlob, err := glob.Compile(strings.TrimSpace(pat))
		if err != nil {
			return err
		}
		j.globs = append(j.globs, lineGlob)
	}

	j.globs = append(j.globs, newGlobs...)

	return nil
}

func (j *JamHubIgnorer) Match(filepath string) bool {
	for _, g := range j.globs {
		match := g.Match(strings.TrimSpace(filepath))
		if match {
			return true
		}
	}
	return false
}
