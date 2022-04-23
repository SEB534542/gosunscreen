package main

import (
	"log"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

func main() {
	rpio.Open()
	defer rpio.Close()
	ls := &LightSensor{
		Pin:         rpio.Pin(23),
		Interval:    time.Duration(time.Minute),
		LightFactor: 12,
		Start:       time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 8, 0, 0, 0, time.Local),
		Stop:        time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 22, 0, 0, 0, time.Local),
		Data:        []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	log.Println("Starting monitor") // TODO: remove(?)
	ls.Monitor()
}
