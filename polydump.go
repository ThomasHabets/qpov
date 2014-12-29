package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ThomasHabets/bsparse/bsp"
	"github.com/ThomasHabets/bsparse/pak"
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
	level, err := bsp.Load(p.Get("maps/e1m1.bsp"))
	if err != nil {
		log.Fatal(err)
	}
	fo, err := os.Create("blah.pov")
	if err != nil {
		log.Fatal(err)
	}
	defer fo.Close()
	startPos := level.StartPos
	if false {
		startPos = bsp.Vertex{
			X: -408,
			Y: -128,
			Z: 128,
		}
	}
	lookAt := startPos
	lookAt.Y += 10
	sky := bsp.Vertex{Z: 1}
	fmt.Fprintf(fo, `#include "colors.inc"
light_source { <%s> color White }
camera {
  location <%s>
  sky <%s>
  look_at <%s>
}
`, startPos.String(), startPos.String(), sky.String(), lookAt.String())
	fmt.Printf("Polygons: %v\n", len(level.Polygons))
	for _, p := range level.Polygons {
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
if false {
		for i, j := 0, len(vs)-1; i < j; i, j = i+1, j-1 {
			vs[i], vs[j] = vs[j], vs[i]
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
}
