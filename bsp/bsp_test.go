package bsp

import (
	"testing"
	"unsafe"
)

func TestSizes(t *testing.T) {
	if got, want := unsafe.Sizeof(RawModel{}), fileModelSize; int(got) != int(want) {
		t.Errorf("Size of Raw Model: got %v, want %v", got, want)
	}
}
