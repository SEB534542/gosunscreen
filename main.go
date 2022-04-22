package main

import (
	"log"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

/* LightSensor represents a physical lightsensor for which data can be collected
through the corresponding GPIO pin.*/
type LightSensor struct {
	Pin         rpio.Pin      // pin for retrieving light value
	Interval    time.Duration // Interval for checking current light in seconds
	LightFactor int           // Factor for correcting the measured analog light value
	Start       time.Time     // Start time for measuring light
	Stop        time.Time     // Stop time for measuring light
	Data        []int         // collected light values
}

func main() {
	rpio.Open()
	defer rpio.Close()
	ls := &LightSensor{
		Pin:      rpio.Pin(23),
		Interval: time.Duration(time.Minute),
		Start:    time.Date(2022, 1, 1, 8, 0, 0, 0, time.Local),
		Stop:     time.Date(2022, 1, 1, 20, 0, 0, 0, time.Local),
	}
	ls.Monitor()
}

func (ls *LightSensor) Monitor() {
	for {
		switch {
		case true:
			light := make(chan int, 2)
			quit := makemonitor(chan bool)
			go sendLight(ls.Pin, ls.Interval, ls.LightFactor, light, quit)
			go receiveLight(light)
		}
	}
}

/*sendLight gathers light from pin every interval and send the light value
on to a channel. This loop runs until the quit chan is closed.*/
func sendLight(pin rpio.Pin, interval time.Duration, lightFactor int, light chan<- int, quit <-chan bool) {
	for {
		select {
		case _, _ = <-quit:
			log.Println("Closing monitorLight") // TODO: remove from log?
			close(light)
			return
		default:
			l, err := getAvgLight(pin, freq)
			l = l / lightFactor
			// Errorhandling
			switch {
			case l == 0:
				log.Printf("No light gathered. Errors: %v", err)
			case err != nil:
				log.Printf("Light gathered: %v with errors: %v", l, err)
			}
			light <- l
			time.Sleep(interval)
		}
	}
}

/*ReceiveLight receives the light and store is in the variable.*/
func receiveLight(light <-chan int) {
	for l := range light {
		log.Printf("Storing light %v...", l)
		// TODO: store light into struct
		// TODO: store light into a log file (via go func?)
	}
}
