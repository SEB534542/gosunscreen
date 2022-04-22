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
	Data        []int         // collected light values
}

func main() {
	rpio.Open()
	defer rpio.Close()
	monitorLight()
}

func (ls *LightSensor) monitor(start, stop) {
	for {
		switch {
		case true:
			light := make(chan int, 2)
			quit := make(chan bool)
			go sendLight(ls.Pin, ls.Interval, ls.LightFactor, light, quit)
			go receiveLight()

		}
	}

}

/*ReceiveLight receives the light and store is in the variable.*/
func receiveLight(light <-chan int) {
	for l := range light {
		log.Printf("Storing light %v...", l)
	}
}
