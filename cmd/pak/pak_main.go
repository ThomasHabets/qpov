// pak allows listing and extracting Quake PAK files.
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
	"io"
	"log"
	"os"
	"strings"

	"github.com/ThomasHabets/qpov/pak"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <pakfiles> command [command args...]\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 2 {
		usage()
		os.Exit(1)
	}

	pakFiles := strings.Split(flag.Arg(0), ",")
	p, err := pak.MultiOpen(pakFiles...)
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()
	switch flag.Arg(1) {
	case "list":
		for _, k := range p.List() {
			fmt.Printf("%s\n", k)
		}
	case "extract":
		fn := flag.Arg(2)
		of, err := os.Create(fn)
		if err != nil {
			log.Fatalf("Opening output file %q: %v", fn, err)
		}
		defer of.Close()
		handle, err := p.Get(fn)
		if err != nil {
			log.Fatalf("Getting %q: %v", fn, err)
		}
		if _, err := io.Copy(of, handle); err != nil {
			os.Remove(of.Name())
			log.Fatalf("Failed to extract %q: %v", fn, err)
		}
	default:
		log.Fatalf("Unknown command %q", flag.Arg(1))
	}
}
