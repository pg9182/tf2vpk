package main

import "github.com/pg9182/tf2vpk/cmd"

// alias tf2vpk='go install ./cmd/tf2vpk && source <(~/go/bin/tf2vpk completion bash) && ~/go/bin/tf2vpk'

func main() {
	cmd.Execute()
}
