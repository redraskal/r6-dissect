package main

import (
	"log"
	"os"

	"github.com/redraskal/r6-dissect/reader"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Specify a match replay file path (*.rec)")
	}
	r, err := reader.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err = PrintHead(*r); err != nil {
		log.Fatal(err)
	}
}
