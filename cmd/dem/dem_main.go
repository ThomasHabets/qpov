package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"

	"github.com/ThomasHabets/bsparse/bsp"
	"github.com/ThomasHabets/bsparse/dem"
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

	df := p.Get(flag.Arg(1))
	d := dem.Open(df)
	oldPos := dem.Vertex{}
	oldView := dem.Vertex{}
	var level *bsp.BSP
	var frame int
	for {
		err := d.Read()
		if err == io.EOF {
			break
		}
		if d.Level == "" {
			continue
		}
		if level == nil {
			level, err = bsp.Load(p.Get(d.Level))
			if err != nil {
				log.Fatalf("Level loading %q: %v", d.Level, err)
			}
			log.Printf("Level start pos: %s", level.StartPos.String())
			d.Pos.X = level.StartPos.X
			d.Pos.Y = level.StartPos.Y
			d.Pos.Z = level.StartPos.Z
		}
		if oldPos != d.Pos {
			fmt.Printf("Frame %d: Pos: %v -> %v, viewAngle %v -> %v\n", frame, oldPos, d.Pos, oldView, d.ViewAngle)
			oldPos = d.Pos
			oldView = d.ViewAngle
			writePOV(fmt.Sprintf("render/frame-%08d.pov", frame), level, d)
			frame++
		}
	}
}

func writePOV(fn string, level *bsp.BSP, d *dem.Demo) {
	fo, err := os.Create(fn)
	if err != nil {
		log.Fatal(err)
	}
	defer fo.Close()

	lookAt := bsp.Vertex{
		X: d.Pos.X + float32(math.Sin(float64(d.ViewAngle.Y))),
		Y: d.Pos.Y + float32(math.Cos(float64(d.ViewAngle.Y))),
		Z: d.Pos.Z,
	}
	pos := bsp.Vertex{
		X: d.Pos.X,
		Y: d.Pos.Y,
		Z: d.Pos.Z,
	}

	fmt.Fprintf(fo, `#include "colors.inc"
light_source { <%s> color White }
camera {
  location <%s>
  sky <0,0,1>
  right <-1,0,0>
  look_at <%s>
}
`, pos.String(), pos.String(), lookAt.String())
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
	}
}
