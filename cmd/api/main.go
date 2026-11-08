package main

import (
	"log"

	"github.com/ajaka/triage/internal"
)

func main() {
	if err := internal.Listen(); err != nil {
		log.Fatal(err)
	}
}
