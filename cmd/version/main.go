package version

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/pg9182/tf2lzham"
	"github.com/pg9182/tf2vpk/cmd/root"
	"github.com/spf13/cobra"
)

var Flags struct {
}

var Command = &cobra.Command{
	Use:   "version",
	Short: "Print the current version",
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func init() {
	root.Command.AddCommand(Command)
}

func main() {
	var vcs struct {
		revision string
		time     time.Time
		modified bool
	}
	var dep struct {
		tf2lzham string
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				v := s.Value
				if len(v) < 20 {
					panic(fmt.Errorf("parse %s %q: too short", s.Key, s.Value))
				}
				vcs.revision = v
			case "vcs.time":
				v, err := time.ParseInLocation(time.RFC3339Nano, s.Value, time.UTC)
				if err != nil {
					panic(fmt.Errorf("parse %s %q: %w", s.Key, s.Value, err))
				}
				vcs.time = v
			case "vcs.modified":
				v, err := strconv.ParseBool(s.Value)
				if err != nil {
					panic(fmt.Errorf("parse %s %q: %w", s.Key, s.Value, err))
				}
				vcs.modified = v
			}
		}
		for _, d := range bi.Deps {
			var s *string
			switch d.Path {
			case "github.com/pg9182/tf2lzham":
				s = &dep.tf2lzham
			}
			if s != nil {
				if d.Replace != nil {
					*s = d.Replace.Path
					if d.Version != "(devel)" {
						*s += " " + d.Replace.Version
					}
				} else {
					if d.Version != "(devel)" {
						*s = d.Version
					}
				}
			}
		}
	}
	if len(vcs.revision) == 0 {
		panic("no version information")
	}

	version := "tf2vpk "
	if vcs.revision != "" {
		version += vcs.revision[:7]
	} else {
		version += "unknown"
	}
	if vcs.modified {
		version += " (modified)"
	}
	fmt.Println(version)

	version = "tf2lzham "
	if dep.tf2lzham != "" {
		version += dep.tf2lzham
	} else {
		version += "unknown"
	}
	if tf2lzham.WebAssembly {
		version += " (wasm)"
	} else {
		version += " (native)"
	}
	fmt.Println(version)
}
