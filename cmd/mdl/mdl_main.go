package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ThomasHabets/bsparse/mdl"
	"github.com/ThomasHabets/bsparse/pak"
)

var (
	model = flag.String("model", "progs/ogre.mdl", "Model to read.")
)

func main() {
	flag.Parse()
	p, err := pak.MultiOpen(flag.Args()...)
	if err != nil {
		log.Fatalf("Failed to open pakfiles %q: %v", flag.Args(), err)
	}

	o, err := p.Get("progs/ogre.mdl")
	if err != nil {
		log.Fatalf("Getting ogre: %v", err)
	}

	m, err := mdl.Load(o)
	if err != nil {
		log.Fatalf("Getting ogre: %v", err)
	}

	of, err := os.Create("model.pov")
	if err != nil {
		log.Fatalf("Model create fail: %v", err)
	}
	defer of.Close()
	for n := range m.Frames {
		fmt.Fprintf(of, "#macro demprefix_ogre_%d(pos, rot)\nunion { %s rotate rot translate pos }\n#end\n", n, m.POVFrameID(n))
	}
}

var randColorState int

func randColor() string {
	randColorState++
	colors := []string{
		"Green",
		"White",
		"Blue",
		"Red",
		"Yellow",
	}
	return colors[randColorState%len(colors)]
}
