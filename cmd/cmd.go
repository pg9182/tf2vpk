package cmd

import (
	"github.com/pg9182/tf2vpk/cmd/root"

	_ "github.com/pg9182/tf2vpk/cmd/cat"
	_ "github.com/pg9182/tf2vpk/cmd/lzham"
	_ "github.com/pg9182/tf2vpk/cmd/verify"
	_ "github.com/pg9182/tf2vpk/cmd/version"
)

func Execute() {
	root.Command.Execute()
}
