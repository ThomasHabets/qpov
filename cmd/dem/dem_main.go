package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"regexp"
	"runtime/pprof"
	"strings"

	"github.com/ThomasHabets/bsparse/bsp"
	"github.com/ThomasHabets/bsparse/dem"
	"github.com/ThomasHabets/bsparse/pak"
)

var (
	cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
	outDir      = flag.String("out", "render", "Output directory.")
	demo        = flag.String("demo", "", "Demo file inside a pak.")
	entities    = flag.Bool("entities", true, "Render entities too.")
	useTextures = flag.Bool("textures", true, "Render textures.")
)

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if len(flag.Args()) == 0 {
		log.Fatal("Usage")
	}
	if *demo == "" {
		log.Fatal("Usage")
	}
	p, err := pak.MultiOpen(flag.Args()...)
	if err != nil {
		log.Fatalf("MultiOpen(%q): %v", flag.Args(), err)
	}
	defer p.Close()

	df, err := p.Get(*demo)
	if err != nil {
		log.Fatalf("Getting %q: %v", *demo, err)
	}
	d := dem.Open(df)
	oldTime := float32(-1.0)
	oldPos := dem.Vertex{}
	oldView := dem.Vertex{}
	var level *bsp.BSP
	var frame int
	for {
		err := d.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Demo error: %v", err)
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
			//d.Pos().X = level.StartPos.X
			//d.Pos().Y = level.StartPos.Y
			//d.Pos().Z = level.StartPos.Z
		}
		if oldTime != d.Time {
			if false {
				fmt.Printf("Frame %d (t=%g): Pos: %v -> %v, viewAngle %v -> %v\n", frame, d.Time, oldPos, d.Pos(), oldView, d.ViewAngle())
			}
			oldView = d.ViewAngle()
			oldPos = d.Pos()
			oldTime = d.Time
			writePOV(path.Join(*outDir, fmt.Sprintf("frame-%08d.pov", frame)), d.Level, level, d)
			frame++
		}
	}
}

func frameName(mf string, frame int) string {
	s := mf

	if true {
		re2 := regexp.MustCompile(`progs/h_`)
		s2 := re2.ReplaceAllString(s, "progs/")
		if _, err := os.Stat(s2 + "/model.inc"); err == nil {
			s = s2
		}
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
	if strings.Contains(m, "suit.mdl") {
		return false
	}
	if strings.Contains(m, "light.mdl") {
		return false
	}
	if strings.Contains(m, "flame2.mdl") {
		return false
	}
	if strings.Contains(m, "w_spike.mdl") {
		return false
	}
	if strings.Contains(m, "missile.mdl") {
		return false
	}
	return true
}

func writePOV(fn string, levelfn string, level *bsp.BSP, d *dem.Demo) {
	ufo, err := os.Create(fn)
	if err != nil {
		log.Fatal("Creating %q: %v", fn, err)
	}
	defer ufo.Close()
	fo := bufio.NewWriter(ufo)
	defer fo.Flush()

	lookAt := bsp.Vertex{
		X: 1,
		Y: 0,
		Z: 0,
	}
	pos := bsp.Vertex{
		X: d.Pos().X,
		Y: d.Pos().Y,
		Z: d.Pos().Z,
	}

	models := []string{}
	if *entities {
		for _, m := range d.ServerInfo.Models {
			if !validModel(m) {
				continue
			}
			models = append(models, fmt.Sprintf(`#include "%s/model.inc"`, m))
		}
	}
	fmt.Fprintf(fo, `#version 3.7;
global_settings {
  assumed_gamma 2.2
}
#include "colors.inc"
#include "progs/soldier.mdl/model.inc"
#include "%s/level.inc"
%s
light_source { <%s> color White }
camera {
  angle 100
  location <0,0,0>
  sky <0,0,1>
  up <0,0,9>
  right <-16,0,0>
  look_at <%s>
  rotate <%f,0,0>
  rotate <0,%f,0>
  rotate <0,0,%f>
  translate <%s>
}
`, levelfn, strings.Join(models, "\n"), pos.String(), lookAt.String(),
		d.ViewAngle().Z,
		d.ViewAngle().X,
		d.ViewAngle().Y,
		//d.ViewAngle.Z, d.ViewAngle.X, d.ViewAngle.Y,
		pos.String())
	if *entities {
		for n, e := range d.Entities {
			if int(d.CameraEnt) == n {
				continue
			}
			if e.Model == 0 {
				// Unused.
				continue
			}
			if int(e.Model) >= len(d.ServerInfo.Models) {
				// TODO: this is dynamic entities?
				continue
			}
			name := d.ServerInfo.Models[e.Model]
			frame := int(e.Frame)
			frame = 0

			// TODO: What's going on here?
			if false {
				switch name {
				case "progs/h_guard.mdl":
					name = "progs/soldier.mdl"
				case "progs/armor.mdl", "progs/spike.mdl", "progs/h_shams.mdl":
					frame = 0
				case "progs/playernl.mdl":
					if frame > 18 {
						frame = 0
					}
				}
			}
			//log.Printf("Entity %d has model %d of %d", n, e.Model, len(d.ServerInfo.Models))
			//log.Printf("  Name: %q", d.ServerInfo.Models[e.Model])
			if validModel(d.ServerInfo.Models[e.Model]) {
				a := e.Angle
				a.X, a.Y, a.Z = a.Z, a.X, a.Y

				// TODO: skin is broken sometimes, just use first one.
				e.Skin = 0
				if *useTextures {
					skinName := path.Join(name, fmt.Sprintf("skin_%v.png", e.Skin))
					fmt.Fprintf(fo, "// Entity %d\n%s(<%s>,<%s>,\"%s\")\n", n, frameName(name, frame), e.Pos.String(), a.String(), skinName)
				} else {
					fmt.Fprintf(fo, "// Entity %d\n%s(<%s>,<%s>)\n", n, frameName(name, frame), e.Pos.String(), a.String())
				}
			}
		}
	}
}

var randColorState int

func randColor() string {
	return "White"
	randColorState++

	// qdqr e1m4 frame 200, polygon 3942 not being drawn correctly.
	if randColorState < 15506 { // 31010 {
		return "White"
	}
	if randColorState > 15510 { // 31021 {
		return "Red"
	}
	colors := []string{
		"Green",
		//"White",
		"Blue",
		//		"Red",
		"Yellow",
		//"Brown",
	}
	return colors[randColorState%len(colors)]
}
