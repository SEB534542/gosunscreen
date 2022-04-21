package main

import (
	"testing"
)

func TestGetLight(t *testing.T) {
	light, err := getLight(rpio.Pin(5))
	t.Log(light, err)
}
