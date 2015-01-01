package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/ThomasHabets/bsparse/mdl"
	"github.com/ThomasHabets/bsparse/pak"
)

var (
	model   = flag.String("model", "progs/ogre.mdl", "Model to read.")
	command = flag.String("c", "show", "Command (convert, show)")
)

func frameName(mf string, frame int) string {
	re := regexp.MustCompile(`[/.-]`)
	return fmt.Sprintf("demprefix_%s_%d", re.ReplaceAllString(mf, "_"), frame)
}

func convert(p pak.MultiPak) {
	errors := []string{}
	for _, mf := range p.List() {
		if path.Ext(mf) != ".mdl" {
			continue
		}
		func() {
			o, err := p.Get(mf)
			if err != nil {
				log.Fatalf("Getting %q: %v", mf, err)
			}

			m, err := mdl.Load(o)
			if err != nil {
				log.Printf("Loading %q: %v", mf, err)
				errors = append(errors, mf)
				return
			}

			fn := fmt.Sprintf("%s.inc", mf)
			of, err := os.Create(fn)
			if err != nil {
				log.Fatalf("Model create of %q fail: %v", fn, err)
			}
			defer of.Close()
			for n := range m.Frames {

				fmt.Fprintf(of, "#macro %s(pos, rot)\n%s\n#end\n", frameName(mf, n), m.POVFrameID(n))
			}
		}()
	}
	fmt.Printf("Failed to convert %d models:\n  %s\n", len(errors), strings.Join(errors, "\n  "))
}

func show(p pak.MultiPak) {
	h, err := p.Get(*model)
	if err != nil {
		log.Fatalf("Unable to get %q: %v", *model, err)
	}

	m, err := mdl.Load(h)
	if err != nil {
		log.Fatalf("Unable to load %q: %v", *model, err)
	}

	fmt.Printf("Filename: %s\n  Triangles: %v\n", *model, len(m.Triangles))
	fmt.Printf("  %6s %16s\n", "Frame#", "Name")
	for n, f := range m.Frames {
		fmt.Printf("  %6d %16s\n", n, f.Name)
	}
}

func triangles(p pak.MultiPak) {
	h, err := p.Get(*model)
	if err != nil {
		log.Fatalf("Unable to get %q: %v", *model, err)
	}

	m, err := mdl.Load(h)
	if err != nil {
		log.Fatalf("Unable to load %q: %v", *model, err)
	}

	for n, _ := range m.Frames {
		fmt.Printf("#macro %s(pos, rot)\n%s\n#end\n", frameName(*model, n), m.POVFrameID(n))
	}
}

func main() {
	flag.Parse()
	p, err := pak.MultiOpen(flag.Args()...)
	if err != nil {
		log.Fatalf("Failed to open pakfiles %q: %v", flag.Args(), err)
	}

	switch *command {
	case "convert":
		convert(p)
	case "pov-tri":
		triangles(p)
	case "show":
		show(p)
	}
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
