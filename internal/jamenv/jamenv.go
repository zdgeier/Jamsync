package jamenv

import (
	"os"
)

type JamEnv int

const (
	Prod JamEnv = iota
	Local
)

func (e JamEnv) String() string {
	switch e {
	case Prod:
		return "prod"
	case Local:
		return "local"
	}
	return "unknown"
}

func Env() JamEnv {
	jamEnvString := os.Getenv("JAM_ENV")
	switch jamEnvString {
	case "local":
		return Local
	default:
		return Prod
	}
}

func User() string {
	return os.Getenv("JAM_USER")
}
