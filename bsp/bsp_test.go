package bsp

import (
	"reflect"
	"testing"
)

func TestSizes(t *testing.T) {
	for _, test := range []struct {
		obj  interface{}
		want int
	}{
		{RawFace{}, fileFaceSize},
		{RawModel{}, fileModelSize},
		{RawTexInfo{}, fileTexInfoSize},
		{RawMipTex{}, fileMiptexSize},
		{Vertex{}, fileVertexSize},
		{RawEdge{}, fileEdgeSize},
	} {
		typ := reflect.TypeOf(test.obj)
		got := typ.Size()
		if int(got) != test.want {
			t.Errorf("Size of %q: got %v, want %v", typ.Name(), got, test.want)
		}
	}
}
