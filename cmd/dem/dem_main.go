package main

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

	"github.com/ThomasHabets/bsparse/bsp"
	"github.com/ThomasHabets/bsparse/dem"
	"github.com/ThomasHabets/bsparse/pak"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	entities   = flag.Bool("entities", true, "Render entities too.")
	verbose    = flag.Bool("v", false, "Verbose output.")
)

func info(p pak.MultiPak, args ...string) {
	/*
		fs := flag.NewFlagSet("info", flag.ExitOnError)
		fs.Parse(args)
		demo := fs.Arg(0)
		df, err := p.Get(demo)
		if err != nil {
			log.Fatalf("Getting %q: %v", demo, err)
		}
		d := dem.Open(df)

		oldTime := float32(-1)
		timeUpdates := 0
		messages := 0
		for {
			err := d.Read()
			messages++
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Demo error: %v", err)
			}
			if oldTime != d.Time {
				timeUpdates++
				oldTime = d.Time
			}
		}
		fmt.Printf("Blocks: %d\n", d.BlockCount)
		fmt.Printf("Messages: %d\n", messages)
		fmt.Printf("Time updates: %d\n", timeUpdates)
	*/
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
	//useTextures := fs.Bool("textures", true, "Render textures.")
	fps := fs.Float64("fps", 25.0, "Frames per second.")
	outDir := fs.String("out", "render", "Output directory.")
	cameraLight := fs.Bool("camera_light", false, "Add camera light.")
	fs.Parse(args)
	demo := fs.Arg(0)

	df, err := p.Get(demo)
	if err != nil {
		log.Fatalf("Getting %q: %v", demo, err)
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
		newState.ViewAngle = block.Header.ViewAngle

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
				if false {
					fmt.Printf("Camera set to %d\n", m.Entity)
				}
			} else if m, ok := msg.(*dem.MsgSpawnBaseline); ok {
				if false {
					fmt.Printf("Spawning %d: %+v\n", m.Entity, newState.ServerInfo.Models[m.Model])
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
			if oldState != nil {
				if *verbose {
					fmt.Printf("Saw time, outputting frames between %g and %g\n",
						oldState.Time,
						newState.Time,
					)
				}
				for _, t := range genTimeFrames(float64(oldState.Time), float64(newState.Time), *fps) {
					generateFrame(p, *outDir, oldState, newState, frameNum, t, *cameraLight)
					anyFrame = true
					frameNum++
				}
			}

			// Only wipe old state if we generate any frame at all.
			if oldState == nil || anyFrame {
				oldState = dem.NewState()
				oldState.Time = newState.Time
				for n := range oldState.Entities {
					oldState.Entities[n] = newState.Entities[n]
				}
				oldState.CameraEnt = newState.CameraEnt
				oldState.ViewAngle = newState.ViewAngle
				oldState.ServerInfo = newState.ServerInfo
				oldState.Level = newState.Level
			}
		}
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

func generateFrame(p pak.MultiPak, outDir string, oldState, newState *dem.State, frameNum int, t float64, cameraLight bool) {
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
	curState := &dem.State{
		Time:       t,
		CameraEnt:  newState.CameraEnt,
		ViewAngle:  interpolateAngle(oldState.ViewAngle, newState.ViewAngle, ival),
		ServerInfo: newState.ServerInfo,
		Level:      newState.Level,
		Entities:   make([]dem.Entity, len(newState.Entities), len(newState.Entities)),
	}
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
			curState.Entities[n].Frame = curState.Entities[n].Frame
			curState.Entities[n].Skin = curState.Entities[n].Skin
			curState.Entities[n].Color = curState.Entities[n].Color
		} else {
			curState.Entities[n].Frame = curState.Entities[n].Frame
			curState.Entities[n].Skin = curState.Entities[n].Skin
			curState.Entities[n].Color = curState.Entities[n].Color
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
	writePOV(path.Join(outDir, fmt.Sprintf("frame-%08d.pov", frameNum)), curState, cameraLight)
}

func frameName(mf string, frame int) string {
	s := mf
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
	return true
}

func writePOV(fn string, state *dem.State, cameraLight bool) {
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
			models = append(models, fmt.Sprintf(`#include "%s/model.inc"`, m))
		}
	}
	fmt.Fprintf(fo, `#version 3.7;
global_settings {
  assumed_gamma 1.0
}
#include "colors.inc"
#include "progs/soldier.mdl/model.inc"
#include "%s/level.inc"
%s
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
%s_0(<0,0,0>,<0,0,0>)
`, state.ServerInfo.Models[0], strings.Join(models, "\n"), lookAt.String(),
		state.ViewAngle.Z,
		state.ViewAngle.X,
		state.ViewAngle.Y,
		//d.ViewAngle.Z, d.ViewAngle.X, d.ViewAngle.Y,
		pos.String(),
		bsp.ModelPrefix(state.ServerInfo.Models[0]),
	)
	re := regexp.MustCompile(`^\*(\d+)$`)
	for _, e := range state.Entities {
		nm := e.Model
		mod := state.ServerInfo.Models[nm]
		m := re.FindStringSubmatch(mod)
		if len(m) == 2 {
			i, _ := strconv.Atoi(m[1])
			fmt.Fprintf(fo, "%s_%d(<%v>,<0,0,0>)\n", bsp.ModelPrefix(state.ServerInfo.Models[0]), i, e.Pos.String())
		}
	}

	if cameraLight {
		fmt.Fprintf(fo, "light_source { <%s> White }\n", pos.String())
	}
	if *entities {
		for n, e := range state.Entities {
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

				// TODO: skin is broken sometimes, just use first one.
				e.Skin = 0
				useTextures := true // TODO
				if useTextures {
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

	p, err := pak.MultiOpen(strings.Split(flag.Arg(0), ",")...)
	if err != nil {
		log.Fatalf("MultiOpen(%q): %v", flag.Arg(0), err)
	}
	defer p.Close()

	switch flag.Arg(1) {
	case "convert":
		convert(p, flag.Args()[2:]...)
	case "info":
		info(p, flag.Args()[2:]...)
	default:
		log.Fatalf("Unknown command %q", flag.Arg(0))
	}
}
