package cmd

import (
	"github.com/pg9182/tf2vpk/cmd/root"

	_ "github.com/pg9182/tf2vpk/cmd/version"
)

func Execute() {
	root.Command.Execute()
}
