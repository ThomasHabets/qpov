package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/ThomasHabets/bsparse/bsp"
	"github.com/ThomasHabets/bsparse/dem"
	"github.com/ThomasHabets/bsparse/pak"
)

var (
	outDir = flag.String("out", "render", "Output directory.")
	demo   = flag.String("demo", "", "Demo file inside a pak.")
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
			if false {
				fmt.Printf("Frame %d: Pos: %v -> %v, viewAngle %v -> %v\n", frame, oldPos, d.Pos, oldView, d.ViewAngle)
			}
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
		if p.Texture[0] == '+' {
			// Animated.
		}
		if p.Texture[0] == '*' {
			// Lava or water.
		}
		if p.Texture == "trigger" {
			continue
		}
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

func frameName(mf string, frame int) string {
	s := mf

	re2 := regexp.MustCompile(`progs/h_`)
	s2 := re2.ReplaceAllString(s, "progs/")
	if _, err := os.Stat(s2 + ".inc"); err == nil {
		s = s2
	}
	re := regexp.MustCompile(`[/.-]`)
	s = re.ReplaceAllString(s, "_")
	return fmt.Sprintf("demprefix_%s_%d", s, frame)
}

func validModel(m string) bool {
	if !strings.HasPrefix(m, "progs/") {
		return false
	}
	if !strings.HasSuffix(m, ".mdl") {
		return false
	}
	if strings.Contains(m, "flame.mdl") {
		return false
	}
	if strings.Contains(m, "eyes.mdl") {
		return false
	}
	if strings.Contains(m, "flame2.mdl") {
		return false
	}
	if strings.Contains(m, "soldier.mdl") {
		return false
	}
	if strings.Contains(m, "w_spike.mdl") {
		return false
	}
	if strings.Contains(m, "h_guard.mdl") {
		return false
	}
	return true
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

	models := []string{}
	for _, m := range d.ServerInfo.Models {
		if !validModel(m) {
			continue
		}
		models = append(models, fmt.Sprintf(`#include "%s.inc"`, m))
	}
	fmt.Fprintf(fo, `#include "colors.inc"
#include "%s"
%s
light_source { <%s> color White }
camera {
  location <0,0,0>
  sky <0,0,1>
  right <-1,0,0>
  look_at <%s>
  rotate <%f,%f,%f>
  translate <%s>
}
`, levelfn, strings.Join(models, "\n"), pos.String(), lookAt.String(),
		d.ViewAngle.X, d.ViewAngle.Y, d.ViewAngle.Z,
		//d.ViewAngle.Z, d.ViewAngle.X, d.ViewAngle.Y,
		pos.String())
	for n, e := range d.Entities {
		if e.Model == 0 {
			// Unused.
			continue
		}
		if int(e.Model) >= len(d.ServerInfo.Models) {
			// TODO: this is dynamic entities?
			continue
		}
		log.Printf("Entity %d has model %d of %d", n, e.Model, len(d.ServerInfo.Models))
		log.Printf("  Name: %q", d.ServerInfo.Models[e.Model])
		if validModel(d.ServerInfo.Models[e.Model]) {
			fmt.Fprintf(fo, "// Entity %d\n", n)
			fmt.Fprintf(fo, "%s(<%s>,<%s>)\n", frameName(d.ServerInfo.Models[e.Model], int(e.Frame)), e.Pos.String(), e.Angle.String())
		}
	}
}

var randColorState int

func randColor() string {
	return "Green"
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
