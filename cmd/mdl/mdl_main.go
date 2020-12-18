// mdl converts Quake MDL files to POV-Ray files.
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
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/ThomasHabets/qpov/pkg/mdl"
	"github.com/ThomasHabets/qpov/pkg/pak"
)

const (
	pngCompressionLevel = 9
)

var (
	pakFiles = flag.String("pak", "", "Comma-separated list of pakfiles to search for resources.")
)

func frameName(mf string, frame int) string {
	re := regexp.MustCompile(`[/.-]`)
	return fmt.Sprintf("demprefix_%s_%d", re.ReplaceAllString(mf, "_"), frame)
}

func convert(p pak.MultiPak, args ...string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -pak <pak0,pak1,...> convert [options]\n", os.Args[0])
		fs.PrintDefaults()
	}
	outDir := fs.String("out", ".", "Output directory.")
	skins := fs.Bool("skins", true, "Use skins.")
	fs.Parse(args)

	errors := []string{}
	os.Mkdir(*outDir, 0755)
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
			var cparts []string
			for _, part := range strings.Split(mf, "/") {
				cparts = append(cparts, part)
				if err := os.Mkdir(path.Join(*outDir, strings.Join(cparts, "/")), 0755); err != nil {
					//log.Printf("Creating model subdir: %v, continuing...", err)
				}
			}
			fn := fmt.Sprintf(path.Join(mf, "model.inc"))
			of, err := os.Create(path.Join(*outDir, fn))
			if err != nil {
				log.Fatalf("Model create of %q fail: %v", fn, err)
			}
			defer of.Close()
			for n := range m.Frames {
				if *skins {
					fmt.Fprintf(of, "#macro %s(pos, rot, skin)\n%s\n#end\n", frameName(mf, n), m.POVFrameID(n, "skin"))
				} else {
					fmt.Fprintf(of, "#macro %s(pos, rot)\n%s\n#end\n", frameName(mf, n), m.POVFrameID(n, ""))
				}
			}

			for n, skin := range m.Skins {
				of, err := os.Create(path.Join(*outDir, mf, fmt.Sprintf("skin_%d.png", n)))
				if err != nil {
					log.Fatalf("Skin create of %q fail: %v", fn, err)
				}
				defer of.Close()
				if err := (&png.Encoder{CompressionLevel: pngCompressionLevel}).Encode(of, skin); err != nil {
					log.Fatalf("Encoding skin to png: %v", err)
				}
			}
		}()
	}
	if len(errors) > 0 {
		fmt.Printf("Failed to convert %d models:\n  %s\n", len(errors), strings.Join(errors, "\n  "))
	}
}

func info(p pak.MultiPak, args ...string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -pak <pak0,pak1,...> info [options] <progs/model.mdl> \n", os.Args[0])
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatalf("Need to specify a model name.")
	}
	model := fs.Arg(0)

	h, err := p.Get(model)
	if err != nil {
		log.Fatalf("Unable to get %q: %v", model, err)
	}

	m, err := mdl.Load(h)
	if err != nil {
		log.Fatalf("Unable to load %q: %v", model, err)
	}

	fmt.Printf("Filename: %s\n", model)
	fmt.Printf("  Triangles: %v\n", len(m.Triangles))
	fmt.Printf("  EyePosition: %v\n", m.Header.EyePosition)
	fmt.Printf("Skins: %v\n", len(m.Skins))
	fmt.Printf("  %6s %16s\n", "Frame#", "Name")
	for n, f := range m.Frames {
		fmt.Printf("  %6d %16s\n", n, f.Name)
	}
}

func triangles(p pak.MultiPak, args ...string) {
	fs := flag.NewFlagSet("pov", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -pak <pak0,pak1,...> pov [options] <progs/model.mdl> \n", os.Args[0])
		fs.PrintDefaults()
	}
	rotate := fs.String("rotate", "0,0,0", "Rotate model.")
	useSkin := fs.Bool("skin", true, "Use texture.")
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatalf("Need to specify a model name.")
	}

	model := fs.Arg(0)

	h, err := p.Get(model)
	if err != nil {
		log.Fatalf("Unable to get %q: %v", model, err)
	}

	m, err := mdl.Load(h)
	if err != nil {
		log.Fatalf("Unable to load %q: %v", model, err)
	}

	for n, _ := range m.Frames {
		skin := "\"" + path.Join(model, "skin_0.png") + "\""
		if !*useSkin {
			skin = ""
		}
		fmt.Printf("#macro %s(pos, rot)\nobject { %s rotate <%s>}\n#end\n", frameName(model, n), m.POVFrameID(n, skin), *rotate)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [global options] command [options]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n  info\n  convert\n  pov\nGlobal options:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

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
	case "pov":
		triangles(p, args...)
	case "info":
		info(p, args...)
	default:
		log.Fatalf("Unknown command %q", cmd)
	}
}
