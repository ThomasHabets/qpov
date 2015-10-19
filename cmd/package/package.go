// This binary makes tar.gz files from directories, embedding and
// deduping symlinked destinations.
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
	"io"
	"os"
	"path"
	"strings"
	"archive/tar"
	"compress/gzip"
	"fmt"
	"log"
	"path/filepath"
)

var (
	out          = flag.String("out", "", "Output file.")
	in           = flag.String("in", "", "Input directory.")
	packageLinks = flag.String("package_links", "_packagelinks", "Directory inside archive to store symlink destinations.")
)

func countDirs(s string) int {
	s = path.Clean(s)
	n := 0
	for {
		s, _ = path.Split(s)
		s = strings.TrimRight(s, fmt.Sprintf("%c", filepath.Separator))
		if len(s) == 0 {
			return n
		}
		n++
	}
}

func pkg(out *tar.Writer, dir string) error {
	linked := make(map[string]string)
	if err := filepath.Walk(dir, func(fn string, fi os.FileInfo, err error) error {
		if fi.Mode().IsRegular() {
			// Regular file: just store it as-is.
			if err := out.WriteHeader(&tar.Header{
				Name:       fn,
				Mode:       0644,
				Uid:        0,
				Gid:        0,
				Size:       fi.Size(),
				ModTime:    fi.ModTime(),
				Typeflag:   tar.TypeReg,
				Uname:      "qpov",
				Gname:      "qpov",
				AccessTime: fi.ModTime(),
				ChangeTime: fi.ModTime(),
			}); err != nil {
				return err
			}
			t, err := os.Open(fn)
			if err != nil {
				return err
			}
			defer t.Close()
			if _, err := io.Copy(out, t); err != nil {
				return err
			}
		} else if 0 != (fi.Mode() & os.ModeSymlink) {
			dest, err := filepath.EvalSymlinks(fn)
			if err != nil {
				return err
			}
			if _, found := linked[dest]; !found {
				// If first occurence, store it.

				// Open real file.
				realFile, err := os.Open(fn)
				if err != nil {
					return err
				}
				defer realFile.Close()
				realStat, err := realFile.Stat()
				if err != nil {
					return err
				}

				// Write file to tarfile.
				newDest := path.Join(*packageLinks, fmt.Sprintf("d%08d%s", len(linked), path.Ext(fn)))

				// Write real file.
				if err := out.WriteHeader(&tar.Header{
					Name:       path.Join(dir, newDest),
					Mode:       0644,
					Uid:        0,
					Gid:        0,
					Size:       realStat.Size(),
					ModTime:    realStat.ModTime(),
					Typeflag:   tar.TypeReg,
					Linkname:   dest,
					Uname:      "qpov",
					Gname:      "qpov",
					AccessTime: fi.ModTime(),
					ChangeTime: fi.ModTime(),
				}); err != nil {
					return fmt.Errorf("writing linked dest: %v", err)
				}
				if _, err := io.Copy(out, realFile); err != nil {
					return fmt.Errorf("writing linked dest data (%q size %d): %v", fn, fi.Size(), err)
				}

				// Create relative path.
				var destPath []string
				for i := 0; i < countDirs(fn)-1; i++ {
					destPath = append(destPath, "..")
				}
				linked[dest] = path.Join(append(destPath, newDest)...)
			}
			if err := out.WriteHeader(&tar.Header{
				Name:       fn,
				Mode:       0644,
				Uid:        0,
				Gid:        0,
				Size:       0,
				ModTime:    fi.ModTime(),
				Typeflag:   tar.TypeSymlink,
				Linkname:   linked[dest],
				Uname:      "qpov",
				Gname:      "qpov",
				AccessTime: fi.ModTime(),
				ChangeTime: fi.ModTime(),
			}); err != nil {
				log.Fatalf("Writing symlink header: %v", err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()
	if len(flag.Args()) != 0 {
		log.Fatalf("Extra args on cmdline: %q", flag.Args())
	}

	fo, err := os.Create(*out)
	if err != nil {
		log.Fatalf("Failed to open output file %q: %v", *out, err)
	}
	if err := func() (err error){
		foz, err := gzip.NewWriterLevel(fo, gzip.BestCompression)
		if err != nil {
			return err
		}
		out := tar.NewWriter(foz)
		if err := pkg(out, *in); err != nil {
			return fmt.Errorf("writing package: %v", err)
		}
		if err := out.Close(); err != nil {
			return err
		}
		if err := foz.Close(); err != nil {
			return err
		}
		if err := fo.Close(); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		if err2 := os.Remove(*out); err2 != nil {
			log.Fatalf("Error deleting failed write. Errors: %v and %v: ", err, err2)
		}
		log.Fatal(err)
	}
}
