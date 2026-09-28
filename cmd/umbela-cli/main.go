package main

import (
	"os"

	"umbela-server/cmd/umbela-cli/cmd"
)

func main(){
	if err:=cmd.Execute(); err != nil {
		os.Exit(1)
	}
}