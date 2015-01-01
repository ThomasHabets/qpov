package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/ThomasHabets/bsparse/bsp"
	"github.com/ThomasHabets/bsparse/pak"
)

var (
	pakFile = flag.String("pak", "", "Pakfile to use.")
)

func main() {
	flag.Parse()

	p, err := pak.MultiOpen(*pakFile)
	if err != nil {
		log.Fatalf("Opening pakfile %q: %v", *pakFile, err)
	}

	mapName := flag.Arg(0)
	b, err := p.Get(mapName)
	if err != nil {
		log.Fatalf("Finding map %q: %v", mapName, err)
	}

	m, err := bsp.LoadRaw(b)
	if err != nil {
		log.Fatalf("Loading map: %v", err)
	}
	fmt.Printf("Vertices: %v\n", len(m.Vertex))
	fmt.Printf("Faces: %v\n", len(m.Face))
	fmt.Printf("Edges: %v\n", len(m.Edge))
	fmt.Printf("LEdges: %v\n", len(m.LEdge))
	fmt.Printf("Vertices: %v\n", len(m.Entities))
	fmt.Printf("MipTexes: %v\n", len(m.MipTex))
	fmt.Printf("TexInfos: %v\n", len(m.TexInfo))
}
