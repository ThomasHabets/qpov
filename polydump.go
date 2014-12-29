package main

import (
	"flag"
"strings"
	"os"
	"log"
	"fmt"

	"github.com/ThomasHabets/bsparse/pak"
	"github.com/ThomasHabets/bsparse/bsp"
)



func main() {
	flag.Parse()
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	p, err := pak.Open(f)
	if err != nil {
		log.Fatal(err)
	}
	for n, e := range p.Entries {
		if false {
			fmt.Printf("%s @%v size=%v\n", n, e.Pos, e.Size)
		}
	}
	bsp, err := bsp.Load(p.Get("maps/e1m1.bsp"))
	if err != nil {
		log.Fatal(err)
	}
	fo, err := os.Create("blah.pov")
	if err != nil {
		log.Fatal(err)
	}
defer 	fo.Close()
	fmt.Fprintf(fo, `#include "colors.inc"
light_source { <-408, -128, 128> color White }
camera {
  location <-408,-128,128>
  look_at <-408,-64,128>
}
`)
	fmt.Printf("Polygons: %v\n", len(bsp.Polygons))
	for _, p := range bsp.Polygons {
		vs := []string{}
		for _, v := range p.Vertex {
			vs = append(vs, fmt.Sprintf("<%f,%f,%f>", v.X, v.Y, v.Z))
		}
		fmt.Fprintf(fo, `polygon {
  %d,
  %s
  finish {
    ambient 0.1
    diffuse 0.6
  }
  pigment { Green }
}
`, len(p.Vertex), strings.Join(vs, ",\n  "))
	}
}
