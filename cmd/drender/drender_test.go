package main

import (
	"testing"
	"time"
)

func TestMakeStats(t *testing.T) {
	makeStats(nil, nil, time.Now())
}
