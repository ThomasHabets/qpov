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

func (p *Pak) Get(fn string) *reader {
	return &reader{
		file:   p.File,
		offset: int64(p.Entries[fn].Pos),
		size:   int64(p.Entries[fn].Size),
	}
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
