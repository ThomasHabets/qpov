package pak

import (
	"reflect"
	"testing"
)

func TestSizes(t *testing.T) {
	for _, test := range []struct {
		obj  interface{}
		want int
	}{
		{fileHeader{}, 12},
		{fileEntry{}, 64},
	} {
		typ := reflect.TypeOf(test.obj)
		got := typ.Size()
		if int(got) != test.want {
			t.Errorf("Size of %q: got %v, want %v", typ.Name(), got, test.want)
		}
	}
}
