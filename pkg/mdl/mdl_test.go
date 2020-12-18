package mdl

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
	"reflect"
	"testing"
)

func TestSizes(t *testing.T) {
	for _, test := range []struct {
		obj  interface{}
		want int
	}{
		{RawHeader{}, 84},
		{TexCoords{}, 12},
		{Triangle{}, 16},
	} {
		typ := reflect.TypeOf(test.obj)
		got := typ.Size()
		if int(got) != test.want {
			t.Errorf("Size of %q: got %v, want %v", typ.Name(), got, test.want)
		}
	}
}
