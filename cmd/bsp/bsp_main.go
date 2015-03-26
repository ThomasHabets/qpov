// bsp converts Quake BSP files to POV-Ray files.
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
	"image"
	"image/png"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/ThomasHabets/qpov/bsp"
	"github.com/ThomasHabets/qpov/pak"
)

const (
	pngCompressionLevel = 9
)

var (
	pakFiles = flag.String("pak", "", "Comma-separated list of pakfiles to search for resources.")
)

func mkdirP(base, mf string) {
	var cparts []string
	for _, part := range strings.Split(mf, "/") {
		cparts = append(cparts, part)
		if err := os.Mkdir(path.Join(base, strings.Join(cparts, "/")), 0755); err != nil {
			//log.Printf("Creating model subdir: %v, continuing...", err)
		}
	}
}

// levelShortname returns just the name portion of the bsp, removing .bsp and directories.
// This is used to find per-map textures.
func levelShortname(l string) string {
	re := regexp.MustCompile(`^(?:.*/)([^/]+).bsp$`)
	m := re.FindStringSubmatch(l)
	if len(m) != 2 {
		return l
	}
	return m[1]
}

// retexture returns a new texture full path name if it finds a replacement texture.
func retexture(retexturePack, mapName string, m bsp.RawMipTex, img image.Image) (string, bool) {
	// First try level-specific retexture.
	fn := path.Join(retexturePack, levelShortname(mapName), m.Name()+".png")
	if _, err := os.Stat(fn); err == nil {
		// log.Printf("Retexturing %q", fn)
		return fn, true
	}

	// Then try global retexture.
	fn = path.Join(retexturePack, m.Name()+".png")
	if _, err := os.Stat(fn); err == nil {
		// log.Printf("Retexturing %q", fn)
		return fn, true
	}

	// No retexture found.
	return "", false
}

func convert(p pak.MultiPak, args ...string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -pak pak0,pak1,... convert [options]\n", os.Args[0])
		fs.PrintDefaults()
	}
	outDir := fs.String("out", ".", "Output directory.")
	retexturePack := fs.String("retexture", "", "Path to retexture pack.")
	flatColor := fs.String("flat_color", "<0.25,0.25,0.25>", "")
	textures := fs.Bool("textures", true, "Use textures.")
	lights := fs.Bool("lights", false, "Export lights.")
	maps := fs.String("maps", ".*", "Maps regex.")
	fs.Parse(args)

	re, err := regexp.Compile(*maps)
	if err != nil {
		log.Fatalf("Maps regex %q invalid: %v", *maps, err)
	}

	//errors := []string{}
	os.Mkdir(*outDir, 0755)
	for _, mf := range p.List() {
		if path.Ext(mf) != ".bsp" {
			continue
		}
		if !re.MatchString(mf) {
			continue
		}
		func() {
			o, err := p.Get(mf)
			if err != nil {
				log.Fatalf("Getting %q: %v", mf, err)
			}

			b, err := bsp.Load(o)
			if err != nil {
				log.Fatalf("Loading %q: %v", mf, err)
			}

			mkdirP(*outDir, mf)
			fn := fmt.Sprintf(path.Join(mf, "level.inc"))
			of, err := os.Create(path.Join(*outDir, fn))
			if err != nil {
				log.Fatalf("Model create of %q fail: %v", fn, err)
			}
			defer of.Close()
			m, err := b.POVTriangleMesh(bsp.ModelMacroPrefix(mf), *textures, *flatColor)
			if err != nil {
				log.Fatalf("Making mesh of %q: %v", mf, err)
			}
			fmt.Fprintln(of, m)
			if *lights {
				fmt.Fprintln(of, b.POVLights())
			}

			if *textures {
				for n, texture := range b.Raw.MipTexData {
					fn := path.Join(*outDir, mf, fmt.Sprintf("texture_%d.png", n))
					retexFn, retex := retexture(*retexturePack, mf, b.Raw.MipTex[n], texture)
					if retex {
						if err := os.Symlink(retexFn, fn); err != nil {
							log.Fatalf("Failed to symlink %q to %q for texture pack: %v", fn, retexFn, err)
						}
						continue
					}
					func() {
						of, err := os.Create(fn)
						if err != nil {
							log.Fatalf("Texture create of %q fail: %v", fn, err)
						}
						defer of.Close()
						if err := (&png.Encoder{CompressionLevel: pngCompressionLevel}).Encode(of, texture); err != nil {
							log.Fatalf("Encoding texture to png: %v", err)
						}
					}()
				}
			}

		}()
	}
}

func info(p pak.MultiPak, args ...string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	//outDir := fs.String("out", ".", "Output directory.")
	//maps := fs.String("maps", ".*", "Regex of maps to convert.")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -pak <pak0,pak1,...> info [options] <maps/eXmX.bsp> \n", os.Args[0])
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatalf("Need to specify a map name.")
	}
	mapName := fs.Arg(0)
	b, err := p.Get(mapName)
	if err != nil {
		log.Fatalf("Finding map %q: %v", mapName, err)
	}

	m, err := bsp.Load(b)
	if err != nil {
		log.Fatalf("Loading map: %v", err)
	}
	fmt.Printf("Vertices: %v\n", len(m.Raw.Vertex))
	fmt.Printf("Faces: %v\n", len(m.Raw.Face))
	fmt.Printf("Edges: %v\n", len(m.Raw.Edge))
	fmt.Printf("LEdges: %v\n", len(m.Raw.LEdge))
	fmt.Printf("Vertices: %v\n", len(m.Raw.Entities))
	fmt.Printf("MipTexes: %v\n", len(m.Raw.MipTex))
	fmt.Printf("TexInfos: %v\n", len(m.Raw.TexInfo))

	fmt.Printf("Model  Faces\n")
	for n, mod := range m.Raw.Models {
		fmt.Printf("%7d %6v\n", n, mod.FaceNum)
	}
}

func pov(p pak.MultiPak, args ...string) {
	fs := flag.NewFlagSet("pov", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -pak <pak0,pak1,...> pov [options] <maps/eXmX.bsp> \n", os.Args[0])
		fs.PrintDefaults()
	}
	lights := fs.Bool("lights", true, "Export lights.")
	flatColor := fs.String("flat_color", "Gray25", "")
	textures := fs.Bool("textures", false, "Use textures.")
	//maps := fs.String("maps", ".*", "Regex of maps to convert.")
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatalf("Need to specify a map name.")
	}
	maps := fs.Arg(0)

	res, err := p.Get(maps)
	if err != nil {
		log.Fatalf("Finding %q: %v", maps, err)
	}

	m, err := bsp.Load(res)
	if err != nil {
		log.Fatalf("Loading %q: %v", maps, err)
	}

	mesh, err := m.POVTriangleMesh(bsp.ModelMacroPrefix(maps), *textures, *flatColor)
	if err != nil {
		log.Fatalf("Error getting mesh: %v", err)
	}
	fmt.Println(mesh)
	if *lights {
		fmt.Println(m.POVLights())
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [global options] command [options]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n  info\n  pov\n  convert\nGlobal options:\n")
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
	case "info":
		info(p, args...)
	case "pov":
		pov(p, args...)
	case "convert":
		convert(p, args...)
	case "help":
		usage()
	default:
		log.Fatalf("Unknown command %q", cmd)
	}
}
