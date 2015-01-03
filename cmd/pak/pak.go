package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ThomasHabets/bsparse/pak"
)

func main() {
	flag.Parse()
	fn := flag.Arg(0)
	f, err := os.Open(fn)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	p, err := pak.Open(f)
	if err != nil {
		log.Fatal(err)
	}
	switch flag.Arg(1) {
	case "list":
		for k := range p.Entries {
			fmt.Printf("%s\n", k)
		}
	case "extract":
		fn := flag.Arg(2)
		of, err := os.Create(fn)
		if err != nil {
			log.Fatalf("Opening output file %q: %v", fn, err)
		}
		defer of.Close()
		handle, err := p.Get(fn)
		if err != nil {
			log.Fatalf("Getting %q: %v", fn, err)
		}
		if _, err := io.Copy(of, handle); err != nil {
			os.Remove(of.Name())
			log.Fatalf("Failed to extract %q: %v", fn, err)
		}
	default:
		log.Fatalf("Unknown command %q", flag.Arg(1))
	}
}
