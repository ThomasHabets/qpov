// Package pak loads Quake PAK files.
//
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
//
package pak

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type fileHeader struct {
	ID            uint32
	Directory     uint32
	DirectorySize uint32
}

type fileEntry struct {
	NameBytes [56]byte
	Offset    uint32
	Size      uint32
}

func (e *fileEntry) Name() string {
	var s string
	for i := 0; i < 56; i++ {
		if e.NameBytes[i] == 0 {
			break
		}
		s = fmt.Sprintf("%s%c", s, rune(e.NameBytes[i]))
	}
	return s
}

type Entry struct {
	Pos  uint32
	Size uint32
}

type Pak struct {
	File    *os.File
	Entries map[string]Entry
}

func (p *Pak) Get(fn string) (*reader, error) {
	entry, found := p.Entries[fn]
	if !found {
		return nil, fmt.Errorf("not found")
	}
	return &reader{
		file:   p.File,
		offset: int64(entry.Pos),
		size:   int64(entry.Size),
	}, nil
}

type reader struct {
	file   *os.File
	offset int64
	size   int64
	pos    int64
}

func (r *reader) Seek(pos int64, rel int) (int64, error) {
	if pos >= r.size {
		pos = r.size - 1
	}
	r.pos = pos
	return pos, nil
}

func (r *reader) Read(data []byte) (int, error) {
	if r.pos >= r.size {
		return 0, io.EOF
	}
	n, err := r.file.ReadAt(data, int64(r.offset+r.pos))
	if err == nil {
		r.pos += int64(n)

		// If read too much.
		if r.pos > r.size {
			n -= int(r.pos - r.size)
			r.pos = r.size
		}
	}
	return n, err
}

type MultiPak []*Pak

func (m MultiPak) List() []string {
	var ret []string
	for _, p := range m {
		for fn := range p.Entries {
			ret = append(ret, fn)
		}
	}
	return ret
}

func MultiOpen(fns ...string) (MultiPak, error) {
	// TODO: don't leak files on error.
	var ret []*Pak
	for _, fn := range fns {
		if fn == "" {
			continue
		}
		f, err := os.Open(fn)
		if err != nil {
			return nil, err
		}
		p, err := Open(f)
		if err != nil {
			return nil, err
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func (m MultiPak) Get(s string) (*reader, error) {
	var r *reader
	var err error
	for i := len(m); i > 0; i-- {
		r, err = m[i-1].Get(s)
		if err == nil {
			return r, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return nil, os.ErrNotExist
}

func (m MultiPak) Close() {
	for _, p := range m {
		p.File.Close()
	}
}

func Open(f *os.File) (*Pak, error) {
	ret := &Pak{
		File:    f,
		Entries: make(map[string]Entry),
	}

	var h fileHeader
	if err := binary.Read(f, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	if _, err := f.Seek(int64(h.Directory), 0); err != nil {
		return nil, err
	}

	var e fileEntry
	for {
		if pos, err := f.Seek(0, os.SEEK_CUR); err != nil {
			return nil, err
		} else if pos >= int64(h.Directory+h.DirectorySize) {
			break
		}
		if err := binary.Read(f, binary.LittleEndian, &e); err != nil {
			return nil, err
		}
		ret.Entries[e.Name()] = Entry{
			Pos:  e.Offset,
			Size: e.Size,
		}
	}
	return ret, nil
}
