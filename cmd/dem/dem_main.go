// dem converts Quake DEM files to POV-Ray files.
package main

// QPov
//
// Copyright (C) Thomas Habets <thomas@habets.se> 2015
// https://github.com/ThomasHabets/qpov
//
//   This program is free software; you can redistribute it and/or modify
//   it under the terms of the GNU General Public License as published by
//   the Free Software Foundation; either version 2 of the License, or
//   (at your option) any later version.
//
//   This program is distributed in the hope that it will be useful,
//   but WITHOUT ANY WARRANTY; without even the implied warranty of
//   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//   GNU General Public License for more details.
//
//   You should have received a copy of the GNU General Public License along
//   with this program; if not, write to the Free Software Foundation, Inc.,
//   51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path"
	"regexp"
	"runtime/pprof"
	"strconv"
	"strings"
	"text/template"

	"github.com/ThomasHabets/qpov/pkg/bsp"
	"github.com/ThomasHabets/qpov/pkg/dem"
	"github.com/ThomasHabets/qpov/pkg/pak"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	entities   = flag.Bool("entities", true, "Render entities too.")
	verbose    = flag.Bool("v", false, "Verbose output.")
	gamma      = flag.Float64("gamma", 1.0, "Gamma to use. 1.0 is good for POV-Ray 3.7, 2.0 for POV-Ray 3.6.")
	pakFiles   = flag.String("pak", "", "Comma-separated list of pakfiles to search for resources.")
	version    = flag.String("version", "3.7", "POV-Ray version to generate data for.")
	prefix     = flag.String("prefix", "", "Add this prefix to all paths to maps and models.")
)

func info(p pak.MultiPak, args ...string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -pak <pak0,pak1,...> info [options] <demofile.dem> \n", os.Args[0])
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatalf("Need to specify a demo name.")
	}

	demo := fs.Arg(0)
	df, err := p.Get(demo)
	if err != nil {
		log.Fatalf("Getting %q: %v", demo, err)
	}
	d := dem.Open(df)

	timeUpdates := 0
	messages := 0
	blockCount := 0
	for {
		block, err := d.ReadBlock()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Demo error: %v", err)
		}
		blockCount++
		msgs, err := block.Messages()
		if err != nil {
			log.Fatalf("Getting messages: %v", err)
		}
		timeUpdatesInBlock := 0
		for _, msg := range msgs {
			messages++
			if _, ok := msg.(*dem.MsgTime); ok {
				timeUpdates++
				timeUpdatesInBlock++
			}
		}
		if timeUpdatesInBlock > 1 {
			fmt.Printf("Block %d had %d time updates\n", blockCount, timeUpdatesInBlock)
		}
	}
	fmt.Printf("Blocks: %d\n", blockCount)
	fmt.Printf("Messages: %d\n", messages)
	fmt.Printf("Time updates: %d\n", timeUpdates)
}

func genTimeFrames(from, to, fps float64) []float64 {
	frames := []float64{}
	frameTime := 1.0 / fps
	frame := int(from / frameTime)
	for {
		frameTime := float64(frame) / fps
		if frameTime >= to {
			return frames
		}
		if frameTime >= from {
			frames = append(frames, frameTime)
		}
		frame++
	}
}

func convert(p pak.MultiPak, args ...string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -pak <pak0,pak1,...> convert [options] <demofile.dem> \n", os.Args[0])
		fs.PrintDefaults()
	}
	radiosity := fs.Bool("radiosity", false, "Use radiosity lighting.")
	fps := fs.Float64("fps", 30.0, "Frames per second.")
	outDir := fs.String("out", "render", "Output directory.")
	cameraLight := fs.Bool("camera_light", false, "Add camera light.")
	outputSound := fs.Bool("output_sound", true, "Output sounds to .wav file.")
	outputPOV := fs.Bool("output_pov", true, "Write POV files.")
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatalf("Need to specify a demo name.")
	}
	demo := fs.Arg(0)

	var df io.Reader
	if _, err := os.Stat(demo); err == nil {
		if dft, err := os.Open(demo); err != nil {
			log.Fatalf("Opening %q: %v", demo, err)
		} else {
			defer dft.Close()
			df = dft
		}
	} else {
		df, err = p.Get(demo)
		if err != nil {
			log.Fatalf("Getting %q: %v", demo, err)
		}
	}
	d := dem.Open(df)

	var oldState *dem.State
	newState := dem.NewState()
	frameNum := 0
	for {
		block, err := d.ReadBlock()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Demo error: %v", err)
		}
		if !newState.CameraSetViewAngle {
			newState.ViewAngle = block.Header.ViewAngle
		}

		seenTime := false
		msgs, err := block.Messages()
		if err != nil {
			log.Fatalf("Getting messages: %v", err)
		}
		for _, msg := range msgs {
			msg.Apply(newState)
			if _, ok := msg.(*dem.MsgTime); ok {
				seenTime = true
			} else if m, ok := msg.(*dem.MsgCameraPos); ok {
				if *verbose {
					fmt.Printf("Camera set to %d\n", m.Entity)
				}
			} else if m, ok := msg.(*dem.MsgIntermission); ok {
				if *verbose {
					fmt.Printf("Intermission: %q\n", m.Text)
				}
			} else if m, ok := msg.(*dem.MsgFinale); ok {
				if *verbose {
					fmt.Printf("Finale: %q\n", m.Text)
				}
			} else if m, ok := msg.(*dem.MsgCameraOrientation); ok {
				if *verbose {
					fmt.Printf("Camera angle set to <%g,%g,%g>\n", m.X, m.Y, m.Z)
				}
			} else if m, ok := msg.(*dem.MsgSpawnBaseline); ok {
				if false {
					fmt.Printf("Spawning %d: %+v\n", m.Entity, newState.ServerInfo.Models[m.Model])
				}
			} else if m, ok := msg.(*dem.MsgPlaySound); ok {
				if false {
					fmt.Printf("Play sound %d: %+v at %f\n", m.Sound, newState.ServerInfo.Sounds[m.Sound], newState.Time)
				}
			} else if m, ok := msg.(*dem.MsgUpdate); ok {
				if false {
					debugEnt := uint16(1)
					if m.Entity == debugEnt {
						fmt.Printf("MOVE Frame %d Ent %d moved from %v %v to %v %v\n", frameNum, m.Entity,
							oldState.Entities[m.Entity].Pos,
							oldState.Entities[m.Entity].Angle,
							newState.Entities[m.Entity].Pos,
							newState.Entities[m.Entity].Angle,
						)
					}
				}
			}
		}

		if seenTime {
			anyFrame := false
			for n := range newState.Entities {
				newState.Entities[n].Visible = newState.SeenEntity[uint16(n)]
			}
			newState.Entities[0].Visible = true // World.
			if oldState != nil {
				if *verbose {
					fmt.Printf("Saw time, outputting frames between %g and %g\n",
						oldState.Time,
						newState.Time,
					)
				}
				for _, t := range genTimeFrames(float64(oldState.Time), float64(newState.Time), *fps) {
					// TODO: Only generate frames if client state is 2.
					if *verbose {
						log.Printf("Generating frame %d", frameNum)
					}
					if *outputPOV {
						generateFrame(p, *outDir, oldState, newState, frameNum, t, *cameraLight, *radiosity)
					}
					anyFrame = true
					frameNum++
				}
			}

			// Only wipe old state if we generate any frame at all.
			if oldState == nil || anyFrame {
				oldState = newState.Copy()
				newState.SeenEntity = make(map[uint16]bool)
			}
		}
	}

	if *outputSound {
		fs, err := os.Create(path.Join(*outDir, "sound.sh"))
		if err != nil {
			log.Fatalf("Failed to open sounds script: %v", err)
		}
		defer fs.Close()
		var files []string
		var delays []string
		for _, s := range newState.Sounds {
			files = append(files, newState.ServerInfo.Sounds[s.Sound.Sound])
			delays = append(delays, fmt.Sprint(s.Time))
		}
		fmt.Fprintf(fs, "sox -M %s -b 16 -c 1 -r 44100 out.wav delay %s remix -p - trim 1.5\n", strings.Join(files, " "), strings.Join(delays, " "))
	}
}

func interpolate(v0, v1 dem.Vertex, t float64) dem.Vertex {
	return dem.Vertex{
		X: float32(float64(v0.X) + t*float64(v1.X-v0.X)),
		Y: float32(float64(v0.Y) + t*float64(v1.Y-v0.Y)),
		Z: float32(float64(v0.Z) + t*float64(v1.Z-v0.Z)),
	}
}

func posAngle(x float32) float32 {
	if x < 0 {
		return x + 360
	}
	return x
}

func interA(a, b, t float64) float64 {
	switch {
	case math.Abs(float64(b-a)) < 180:
		return a + t*(b-a)
	case a > b:
		b += 360
		return a + t*(b-a)
	default:
		a += 360
		return a + t*(b-a)
	}
}

func interpolateAngle(v0, v1 dem.Vertex, t float64) dem.Vertex {
	a := dem.Vertex{
		X: posAngle(v0.X),
		Y: posAngle(v0.Y),
		Z: posAngle(v0.Z),
	}
	b := dem.Vertex{
		X: posAngle(v1.X),
		Y: posAngle(v1.Y),
		Z: posAngle(v1.Z),
	}

	var ret dem.Vertex
	ret.X = float32(interA(float64(a.X), float64(b.X), t))
	ret.Y = float32(interA(float64(a.Y), float64(b.Y), t))
	ret.Z = float32(interA(float64(a.Z), float64(b.Z), t))
	if true {
		for ret.X > 180 {
			ret.X -= 360
		}
		for ret.Y > 180 {
			ret.Y -= 360
		}
		for ret.Z > 180 {
			ret.Z -= 360
		}
	}
	return ret
}

// return true if object is moving too fast to be interpolated, and should instead jump into place.
// This is for teleportations.
func tooFast(s0, s1 *dem.State) bool {
	const maxSpeed = 50.0
	const maxAngleSpeed = 45.0
	dx := math.Abs(float64(s0.Entities[s0.CameraEnt].Pos.X - s1.Entities[s1.CameraEnt].Pos.X))
	dy := math.Abs(float64(s0.Entities[s0.CameraEnt].Pos.Y - s1.Entities[s1.CameraEnt].Pos.Y))
	dz := math.Abs(float64(s0.Entities[s0.CameraEnt].Pos.Z - s1.Entities[s1.CameraEnt].Pos.Z))
	if speed := math.Sqrt(dx*dx + dy*dy + dz*dz); speed > maxSpeed {
		if false {
			fmt.Printf("Speed %g %v %v\n", speed, s0.Entities[s0.CameraEnt].Pos, s1.Entities[s1.CameraEnt].Pos)
		}
		return true
	}
	dx = math.Abs(float64(s0.ViewAngle.X - s1.ViewAngle.X))
	dy = math.Abs(float64(s0.ViewAngle.Y - s1.ViewAngle.Y))
	dz = math.Abs(float64(s0.ViewAngle.Z - s1.ViewAngle.Z))
	if speed := math.Sqrt(dx*dx + dy*dy + dz*dz); speed > maxAngleSpeed {
		if false {
			fmt.Printf("Angular speed %g\n", speed)
		}
		return true
	}
	return false
}

// generateFrame generates frame number `frameNum`
func generateFrame(p pak.MultiPak, outDir string, oldState, newState *dem.State, frameNum int, t float64, cameraLight, radiosity bool) {
	if newState.ServerInfo.Models == nil {
		return
	}
	if newState.Level == nil {

		bl, err := p.Get(newState.ServerInfo.Models[0])
		if err != nil {
			log.Fatalf("Looking up %q: %v", newState.ServerInfo.Models[0], err)
		}
		newState.Level, err = bsp.Load(bl)
		if err != nil {
			log.Fatalf("Level loading %q: %v", newState.ServerInfo.Models[0], err)
		}
		// TODO
		//log.Printf("Level start pos: %s", newState.Level.StartPos.String())
		//d.Pos().X = level.StartPos.X
		//d.Pos().Y = level.StartPos.Y
		//d.Pos().Z = level.StartPos.Z
	}
	ival := float64(t-oldState.Time) / float64(newState.Time-oldState.Time)
	curState := newState.Copy()
	curState.Time = t
	curState.ViewAngle = interpolateAngle(oldState.ViewAngle, newState.ViewAngle, ival)
	if tooFast(oldState, newState) { //, float64(newState.Time-oldState.Time)) {
		if false {
			fmt.Printf("Frame %d: Moving too fast, snapping.\n", frameNum)
		}
		if ival < 0.5 {
			curState.ViewAngle = oldState.ViewAngle
			curState.Entities[curState.CameraEnt].Pos = oldState.Entities[curState.CameraEnt].Pos
		} else {
			curState.ViewAngle = newState.ViewAngle
			curState.Entities[curState.CameraEnt].Pos = newState.Entities[curState.CameraEnt].Pos
		}
	}
	for n := range oldState.Entities {
		if oldState.Entities[n].Model != newState.Entities[n].Model {
			// If model has changed, choose nearest and stop.
			if ival < 0.5 {
				curState.Entities[n] = oldState.Entities[n]
			} else {
				curState.Entities[n] = newState.Entities[n]
			}
			continue
		}
		curState.Entities[n].Model = oldState.Entities[n].Model
		curState.Entities[n].Pos = interpolate(oldState.Entities[n].Pos, newState.Entities[n].Pos, ival)
		if false && n == 8 {
			fmt.Printf("Frame %d, ent %d %v (%v -> %v by %g)\n", frameNum, n,
				curState.Entities[n].Pos.String(),
				oldState.Entities[n].Pos.String(),
				newState.Entities[n].Pos.String(),
				ival,
			)
		}
		curState.Entities[n].Angle = interpolateAngle(oldState.Entities[n].Angle, newState.Entities[n].Angle, ival)
		if ival < 0.5 {
			curState.Entities[n].Frame = oldState.Entities[n].Frame
			curState.Entities[n].Skin = oldState.Entities[n].Skin
			curState.Entities[n].Color = oldState.Entities[n].Color
		} else {
			curState.Entities[n].Frame = newState.Entities[n].Frame
			curState.Entities[n].Skin = newState.Entities[n].Skin
			curState.Entities[n].Color = newState.Entities[n].Color
		}
	}
	if *verbose {
		fmt.Printf("Frame %d (t=%g): Pos: %v (%v -> %v, %g), viewAngle %v (%v -> %v)\n", frameNum, curState.Time,
			curState.Entities[curState.CameraEnt].Pos,
			oldState.Entities[curState.CameraEnt].Pos,
			newState.Entities[curState.CameraEnt].Pos,
			ival,
			curState.ViewAngle,
			oldState.ViewAngle,
			newState.ViewAngle,
		)
	}
	writePOV(path.Join(outDir, fmt.Sprintf("frame-%08d.pov", frameNum)), newState.ServerInfo.Models[0], curState, cameraLight, radiosity)
}

func frameName(mf string, frame int) string {
	s := mf
	re := regexp.MustCompile(`[/.-]`)
	s = re.ReplaceAllString(s, "_")
	return fmt.Sprintf("demprefix_%s_%d", s, frame)
}

func validModel(m string) bool {
	// These have grouped frames. Not yet handled.
	if strings.Contains(m, "flame.mdl") {
		return false
	}
	if strings.Contains(m, "flame2.mdl") {
		return false
	}
	if strings.Contains(m, "w_spike.mdl") {
		return false
	}

	if strings.HasSuffix(m, ".mdl") {
		return true
	}
	if strings.HasSuffix(m, ".bsp") {
		return true
	}
	return false
}

func writePOV(fn, texturesPath string, state *dem.State, cameraLight, radiosity bool) {
	ufo, err := os.Create(fn)
	if err != nil {
		log.Fatalf("Creating %q: %v", fn, err)
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
		X: state.Entities[state.CameraEnt].Pos.X,
		Y: state.Entities[state.CameraEnt].Pos.Y,
		Z: state.Entities[state.CameraEnt].Pos.Z,
	}

	models := []string{}
	if *entities {
		for _, m := range state.ServerInfo.Models {
			if !validModel(m) {
				continue
			}
			if strings.HasSuffix(m, ".mdl") {
				models = append(models, fmt.Sprintf(`%s/model.inc`, m))
			}
			if strings.HasSuffix(m, ".bsp") {
				models = append(models, fmt.Sprintf(`%s/level.inc`, m))
			}
		}
	}
	eyeLevel := bsp.Vertex{
		Z: 10,
	}

	tmpl := template.Must(template.New("header").Parse(`
{{$root := .}}
#version {{.Version}};
#include "rad_def.inc"
global_settings {
  assumed_gamma {{.Gamma}}
  {{ if .Radiosity }}radiosity { Rad_Settings(Radiosity_Normal,off,off)}{{ end }}
}
#include "{{.Prefix}}progs/soldier.mdl/model.inc"
#include "{{.Prefix}}{{.Level}}/level.inc"
{{ range .Models }}#include "{{$root.Prefix}}{{ . }}"
{{ end }}
camera {
  angle 100
  location <0,0,0>
  sky <0,0,1>
  up <0,0,9>
  right <-16,0,0>
  look_at <{{.LookAt}}>
  rotate <{{.AngleX}},0,0>
  rotate <0,{{.AngleY}},0>
  rotate <0,0,{{.AngleZ}}>
  translate <{{.Pos}}>
  translate <{{.EyeLevel}}>
}
`))
	if err := tmpl.Execute(fo, struct {
		Gamma                  float64
		Prefix                 string
		Version                string
		Radiosity              bool
		AngleX, AngleY, AngleZ float64
		Pos                    string
		LookAt                 string
		Level                  string
		EyeLevel               string
		Models                 []string
	}{
		Prefix:    *prefix,
		Version:   *version,
		Gamma:     *gamma,
		Radiosity: radiosity,
		Level:     state.ServerInfo.Models[0],
		Models:    models,
		LookAt:    lookAt.String(),
		AngleX:    float64(state.ViewAngle.Z),
		AngleY:    float64(state.ViewAngle.X),
		AngleZ:    float64(state.ViewAngle.Y),
		Pos:       pos.String(),
		EyeLevel:  eyeLevel.String(),
	}); err != nil {
		log.Fatalf("Executing template: %v", err)
	}
	re := regexp.MustCompile(`^\*(\d+)$`)
	for _, e := range state.Entities {
		if !e.Visible {
			continue
		}
		nm := e.Model
		mod := state.ServerInfo.Models[nm]
		m := re.FindStringSubmatch(mod)
		if len(m) == 2 {
			i, _ := strconv.Atoi(m[1])
			fmt.Fprintf(fo, "%s_%d(<%v>,<0,0,0>,\"%s\")\n", bsp.ModelMacroPrefix(state.ServerInfo.Models[0]), i, e.Pos.String(), *prefix+texturesPath)
		}
	}

	if cameraLight {
		fmt.Fprintf(fo, "light_source { <%s> rgb<1,1,1> translate <%s> }\n", pos.String(), eyeLevel.String())
	}
	if *entities {
		for n, e := range state.Entities {
			if !e.Visible {
				continue
			}
			if int(state.CameraEnt) == n {
				continue
			}
			if e.Model == 0 {
				// Unused.
				continue
			}
			if int(e.Model) >= len(state.ServerInfo.Models) {
				// TODO: this is dynamic entities?
				continue
			}
			name := state.ServerInfo.Models[e.Model]
			frame := int(e.Frame)

			//log.Printf("Entity %d has model %d of %d", n, e.Model, len(d.ServerInfo.Models))
			//log.Printf("  Name: %q", d.ServerInfo.Models[e.Model])
			if validModel(state.ServerInfo.Models[e.Model]) {
				a := e.Angle
				a.X, a.Y, a.Z = a.Z, a.X, a.Y
				modelName := state.ServerInfo.Models[e.Model]
				if strings.HasSuffix(modelName, ".mdl") {
					useTextures := true // TODO
					if useTextures {
						skinName := path.Join(name, fmt.Sprintf("skin_%v.png", e.Skin))
						fmt.Fprintf(fo, "// Entity %d\n%s(<%s>,<%s>,\"%s\")\n", n, frameName(name, frame), e.Pos.String(), a.String(), *prefix+skinName)
					} else {
						fmt.Fprintf(fo, "// Entity %d\n%s(<%s>,<%s>)\n", n, frameName(name, frame), e.Pos.String(), a.String())
					}
				} else if strings.HasSuffix(state.ServerInfo.Models[e.Model], ".bsp") {
					fmt.Fprintf(fo, "// BSP Entity %d\n%s_0(<%s>,<%s>, \"%s\")\n", n, bsp.ModelMacroPrefix(modelName), e.Pos.String(), a.String(), *prefix+modelName)
				}

			}
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [global options] command [options]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n  info\n  convert\nGlobal options:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	pf := strings.Split(*pakFiles, ",")
	p, err := pak.MultiOpen(pf...)
	if err != nil {
		log.Fatalf("Opening pakfiles %q: %v", pf, err)
	}
	defer p.Close()

	if flag.NArg() == 0 {
		usage()
		log.Fatalf("Need to specify a command.")
	}

	cmd := flag.Arg(0)
	args := flag.Args()[1:]
	switch cmd {
	case "convert":
		convert(p, args...)
	case "info":
		info(p, args...)
	default:
		log.Fatalf("Unknown command %q", cmd)
	}
}
