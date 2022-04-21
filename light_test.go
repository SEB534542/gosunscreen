package main

import (
	"testing"

	"github.com/stianeikeland/go-rpio/v4"
)

const (
	lightSensor = rpio.Pin(23)
	freq        = 10
)

// func TestGetLight(t *testing.T) {
// 	rpio.Open()
// 	defer rpio.Close()
// 	light, err := getLight(lightSensor)
// 	t.Log(light, err)
// }

func TestGetAvgLight(t *testing.T) {
	rpio.Open()
	defer rpio.Close()
	light, err := getAvgLight(lightSensor, freq)
	t.Log(light, err)
}
