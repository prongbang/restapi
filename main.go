package main

import (
	"log"

	"github.com/prongbang/restapi/cmd"
)

func main() {
	if err := cmd.RootCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
