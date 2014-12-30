package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/ThomasHabets/bsparse/bsp"
	"github.com/ThomasHabets/bsparse/dem"
	"github.com/ThomasHabets/bsparse/pak"
)

var (
	outDir = flag.String("out", "render", "Output directory.")
	demo = flag.String("demo", "", "Demo file inside a pak.")
)

func main() {
	flag.Parse()

	p, err := pak.MultiOpen(flag.Args()...)
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()

	df, err := p.Get(*demo)
	if err != nil {
		log.Fatal(err)
	}
	d := dem.Open(df)
	oldPos := dem.Vertex{}
	oldView := dem.Vertex{}
	var level *bsp.BSP
	var frame int
	var levelfn string
	for {
		err := d.Read()
		if err == io.EOF {
			break
		}
		if d.Level == "" {
			continue
		}
		if level == nil {
			bl, err := p.Get(d.Level)
			if err != nil {
				log.Fatal(err)
			}
			level, err = bsp.Load(bl)
			if err != nil {
				log.Fatalf("Level loading %q: %v", d.Level, err)
			}
			log.Printf("Level start pos: %s", level.StartPos.String())
			d.Pos.X = level.StartPos.X
			d.Pos.Y = level.StartPos.Y
			d.Pos.Z = level.StartPos.Z
			levelfn = fmt.Sprintf("%s.inc", path.Base(d.Level))
			writeLevel(path.Join(*outDir, levelfn), level)
		}
		if oldPos != d.Pos {
			fmt.Printf("Frame %d: Pos: %v -> %v, viewAngle %v -> %v\n", frame, oldPos, d.Pos, oldView, d.ViewAngle)
			oldPos = d.Pos
			oldView = d.ViewAngle
			writePOV(path.Join(*outDir, fmt.Sprintf("frame-%08d.pov", frame)), levelfn, level, d)
			frame++
		}
	}
}

func writeLevel(fn string, level *bsp.BSP) {
	fo, err := os.Create(fn)
	if err != nil {
		log.Fatal(err)
	}
	defer fo.Close()
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
  pigment { %s }
}
`, len(p.Vertex), strings.Join(vs, ",\n  "), randColor())
	}
}

func writePOV(fn string, levelfn string, level *bsp.BSP, d *dem.Demo) {
	fo, err := os.Create(fn)
	if err != nil {
		log.Fatal(err)
	}
	defer fo.Close()

	lookAt := bsp.Vertex{
		X: 1,
		Y: 0,
		Z: 0,
	}
	pos := bsp.Vertex{
		X: d.Pos.X,
		Y: d.Pos.Y,
		Z: d.Pos.Z,
	}

	fmt.Fprintf(fo, `#include "colors.inc"
#include "%s"
light_source { <%s> color White }
camera {
  location <0,0,0>
  sky <0,0,1>
  right <-1,0,0>
  look_at <%s>
  rotate <%f,%f,%f>
  translate <%s>
}
`, levelfn, pos.String(), lookAt.String(),
		//d.ViewAngle.Z, d.ViewAngle.Y, d.ViewAngle.X,
		d.ViewAngle.Z, d.ViewAngle.X, d.ViewAngle.Y,
		pos.String())
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
