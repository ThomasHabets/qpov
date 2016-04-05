package main

import (
	"testing"
	"time"
)

const tf = "2006-01-02"

func pt(s string) time.Time {
	r, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return r
}

func TestFlip(t *testing.T) {
	in := [][]tsInt{
		{
			{time: pt("2016-01-01"), value: 1},
			{time: pt("2016-01-02"), value: 2},
			{time: pt("2016-02-01"), value: 10},
			{time: pt("2016-02-02"), value: 11},
		},
		{
			{time: pt("2016-01-03"), value: 3},
			{time: pt("2016-01-04"), value: 4},
			{time: pt("2016-01-05"), value: 5},
			{time: pt("2016-01-06"), value: 6},
		},
	}
	out := flip(in)
	return
	// TODO: make actual tests.
	for _, o := range out {
		t.Errorf("%v %v", o.time, o.values)
	}
}
