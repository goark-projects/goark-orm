package main

import (
	"os"

	"goark.dev/orm/internal/ormcli"
)

var version = ormcli.Version

func main() {
	os.Exit(ormcli.Command{
		Out:     os.Stdout,
		Err:     os.Stderr,
		Version: version,
	}.Run(os.Args[1:]))
}
