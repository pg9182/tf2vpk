package cmd

import (
	"github.com/pg9182/tf2vpk/cmd/root"

	_ "github.com/pg9182/tf2vpk/cmd/chflg"
	_ "github.com/pg9182/tf2vpk/cmd/filter"
	_ "github.com/pg9182/tf2vpk/cmd/get"
	_ "github.com/pg9182/tf2vpk/cmd/init"
	_ "github.com/pg9182/tf2vpk/cmd/list"
	_ "github.com/pg9182/tf2vpk/cmd/lzham"
	_ "github.com/pg9182/tf2vpk/cmd/rm"
	_ "github.com/pg9182/tf2vpk/cmd/tarzip"
	_ "github.com/pg9182/tf2vpk/cmd/unpack"
	_ "github.com/pg9182/tf2vpk/cmd/verify"
	_ "github.com/pg9182/tf2vpk/cmd/version"
	_ "github.com/pg9182/tf2vpk/cmd/vpkfiles"
	_ "github.com/pg9182/tf2vpk/cmd/vpkflags"
)

func Execute() {
	root.Command.Execute()
}
