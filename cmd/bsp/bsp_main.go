package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ThomasHabets/bsparse/bsp"
	"github.com/ThomasHabets/bsparse/pak"
)

var (
	pakFile = flag.String("pak", "", "Pakfile to use.")
	command = flag.String("c", "", "Command (pov-tri, info)")
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

	m, err := bsp.Load(b)
	if err != nil {
		log.Fatalf("Loading map: %v", err)
	}
	switch *command {
	case "info":
		fmt.Fprintf(os.Stderr, "Vertices: %v\n", len(m.Raw.Vertex))
		fmt.Fprintf(os.Stderr, "Faces: %v\n", len(m.Raw.Face))
		fmt.Fprintf(os.Stderr, "Edges: %v\n", len(m.Raw.Edge))
		fmt.Fprintf(os.Stderr, "LEdges: %v\n", len(m.Raw.LEdge))
		fmt.Fprintf(os.Stderr, "Vertices: %v\n", len(m.Raw.Entities))
		fmt.Fprintf(os.Stderr, "MipTexes: %v\n", len(m.Raw.MipTex))
		fmt.Fprintf(os.Stderr, "TexInfos: %v\n", len(m.Raw.TexInfo))
	case "pov-tri":
		mesh, err := m.POVTriangleMesh()
		if err != nil {
			log.Fatalf("Error getting mesh: %v", err)
		}
		fmt.Println(mesh)
	default:
		log.Fatalf("Unknown command %q", *command)
	}
}
