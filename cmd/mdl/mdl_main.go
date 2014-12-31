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
	model = flag.String("model", "progs/ogre.mdl", "Model to read.")
)

func frameName(mf string, frame int) string {
	re := regexp.MustCompile(`[/.-]`)
	return fmt.Sprintf("demprefix_%s_%d", re.ReplaceAllString(mf, "_"), frame)
}

func main() {
	flag.Parse()
	p, err := pak.MultiOpen(flag.Args()...)
	if err != nil {
		log.Fatalf("Failed to open pakfiles %q: %v", flag.Args(), err)
	}

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

				fmt.Fprintf(of, "#macro %s(pos, rot)\nunion { %s rotate rot translate pos }\n#end\n", frameName(mf, n), m.POVFrameID(n))
			}
		}()
	}
	fmt.Printf("Failed to convert %d models:\n  %s\n", len(errors), strings.Join(errors, "\n  "))
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
