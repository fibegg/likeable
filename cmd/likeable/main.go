package main

import (
	"log"

	"github.com/fibegg/likeable/internal/likeable"
)

func main() {
	if err := likeable.Run(); err != nil {
		log.Fatal(err)
	}
}
