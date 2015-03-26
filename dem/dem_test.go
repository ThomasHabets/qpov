package dem

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
	"bytes"
	"fmt"
	"io"
	"reflect"
	"testing"
)

func TestSizes(t *testing.T) {
	for _, test := range []struct {
		obj  interface{}
		want int
	}{
		{BlockHeader{}, 16},
	} {
		typ := reflect.TypeOf(test.obj)
		got := typ.Size()
		if int(got) != test.want {
			t.Errorf("Size of %q: got %v, want %v", typ.Name(), got, test.want)
		}
	}
}

func TestRead(t *testing.T) {
	for name, test := range map[string]struct {
		input     []byte
		f         func(io.Reader) (interface{}, error)
		output    interface{}
		remaining int
		fail      bool
	}{
		// readInt8
		"positive int8": {
			input:  []byte{123},
			f:      func(r io.Reader) (interface{}, error) { return readInt8(r) },
			output: interface{}(123),
		},
		"negative int8": {
			input:     []byte{0xff, 1, 1, 1},
			f:         func(r io.Reader) (interface{}, error) { return readInt8(r) },
			output:    interface{}(-1),
			remaining: 3,
		},
		"short int8": {
			input: []byte{},
			f:     func(r io.Reader) (interface{}, error) { return readInt8(r) },
			fail:  true,
		},

		// readUint8
		"low uint8": {
			input:  []byte{123},
			f:      func(r io.Reader) (interface{}, error) { return readUint8(r) },
			output: interface{}(123),
		},
		"high uint8": {
			input:     []byte{0xff, 1, 2, 3},
			f:         func(r io.Reader) (interface{}, error) { return readUint8(r) },
			output:    interface{}(255),
			remaining: 3,
		},
		"short uint8": {
			input: []byte{},
			f:     func(r io.Reader) (interface{}, error) { return readUint8(r) },
			fail:  true,
		},

		// readInt16
		"positive int16": {
			input:  []byte{0x12, 0x23},
			f:      func(r io.Reader) (interface{}, error) { return readInt16(r) },
			output: interface{}(0x2312),
		},
		"negative int16": {
			input:     []byte{0xfe, 0xff, 1, 1},
			f:         func(r io.Reader) (interface{}, error) { return readInt16(r) },
			output:    interface{}(-2),
			remaining: 2,
		},
		"short int16": {
			input: []byte{123},
			f:     func(r io.Reader) (interface{}, error) { return readInt16(r) },
			fail:  true,
		},

		// readCoord
		"readCoord: positive": {
			input:  []byte{1, 2},
			f:      func(r io.Reader) (interface{}, error) { return readCoord(r) },
			output: interface{}(64.125),
		},
		"readCoord: negative": {
			input:     []byte{0xfe, 0xff, 1, 1},
			f:         func(r io.Reader) (interface{}, error) { return readCoord(r) },
			output:    interface{}(-0.25),
			remaining: 2,
		},
		"readCoord: short": {
			input: []byte{123},
			f:     func(r io.Reader) (interface{}, error) { return readCoord(r) },
			fail:  true,
		},

		// readAngle
		"readAngle: positive": {
			input:  []byte{100},
			f:      func(r io.Reader) (interface{}, error) { return readAngle(r) },
			output: interface{}(140.625),
		},
		"readAngle: negative": {
			input:     []byte{0xC0, 1, 2, 3},
			f:         func(r io.Reader) (interface{}, error) { return readAngle(r) },
			output:    interface{}(-90),
			remaining: 3,
		},
		"readAngle: short": {
			input: []byte{},
			f:     func(r io.Reader) (interface{}, error) { return readAngle(r) },
			fail:  true,
		},

		// readUint16
		"low uint16": {
			input:  []byte{0x12, 0x23},
			f:      func(r io.Reader) (interface{}, error) { return readUint16(r) },
			output: interface{}(0x2312),
		},
		"high uint16": {
			input:     []byte{0xfe, 0xff, 1, 2, 3},
			f:         func(r io.Reader) (interface{}, error) { return readUint16(r) },
			output:    interface{}(0xfffe),
			remaining: 3,
		},
		"short uint16": {
			input: []byte{123},
			f:     func(r io.Reader) (interface{}, error) { return readUint16(r) },
			fail:  true,
		},

		// readUint32
		"low uint32": {
			input:  []byte{0x12, 0x23, 0x34, 0x45},
			f:      func(r io.Reader) (interface{}, error) { return readUint32(r) },
			output: interface{}(0x45342312),
		},
		"high uint32": {
			input:     []byte{0xff, 0xfe, 0xfd, 0xfc, 1, 2, 3},
			f:         func(r io.Reader) (interface{}, error) { return readUint32(r) },
			output:    interface{}(0xfcfdfeff),
			remaining: 3,
		},
		"short uint32": {
			input: []byte{1, 2, 3},
			f:     func(r io.Reader) (interface{}, error) { return readUint32(r) },
			fail:  true,
		},

		// readString
		"readString: zero-length": {
			input:  []byte{0},
			f:      func(r io.Reader) (interface{}, error) { return readString(r) },
			output: interface{}(""),
		},
		"readString: normal": {
			input:     []byte{0x41, 0x42, 0x43, 0, 1, 2},
			f:         func(r io.Reader) (interface{}, error) { return readString(r) },
			output:    interface{}("ABC"),
			remaining: 2,
		},
		"readString: no null": {
			input: []byte{0x41, 0x42, 0x43},
			f:     func(r io.Reader) (interface{}, error) { return readString(r) },
			fail:  true,
		},

		// readFloat
		"readFloat: positive": {
			input:  []byte{56, 180, 150, 73},
			f:      func(r io.Reader) (interface{}, error) { return readFloat(r) },
			output: interface{}("1.234567e+06"),
		},
		"readFloat: negative": {
			input:     []byte{223, 71, 63, 196, 2, 3, 1, 2},
			f:         func(r io.Reader) (interface{}, error) { return readFloat(r) },
			output:    interface{}(-765.123),
			remaining: 4,
		},
		"readFloat: short": {
			input: []byte{1, 2, 3},
			f:     func(r io.Reader) (interface{}, error) { return readFloat(r) },
			fail:  true,
		},
	} {
		buf := bytes.NewBuffer(test.input)
		val, err := test.f(buf)
		if test.fail == true {
			if err == nil {
				t.Errorf("%s: Want to fail, didn't", name)
			}
		} else {
			if err != nil {
				t.Errorf("%s: Failed to parse %v: %v", name, test.input, err)
			} else {
				if got, want := val, test.output; fmt.Sprintf("%v", got) != fmt.Sprintf("%+v", want) {
					t.Errorf("%s: got %v, want %v", name, got, want)
				}
				if got, want := buf.Len(), test.remaining; got != want {
					t.Errorf("%s: got %v remaining in buffer, want %v", name, got, want)
				}
			}
		}
	}
}
