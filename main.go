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
	// TODO: specify length of data
}

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

func (ls *LightSensor) Monitor() {
	for {
		switch {
		case time.Now().After(ls.Stop):
			log.Println("Reset Start and Stop to tomorrow") // TODO: remove(?)
			// Reset Start and Stop to tomorrow
			ls.Start = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day()+1, ls.Start.Hour(), ls.Start.Minute(), 0, 0, time.Local)
			ls.Stop = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day()+1, ls.Stop.Hour(), ls.Stop.Minute(), 0, 0, time.Local)
			fallthrough
		case time.Now().Before(ls.Start):
			log.Println("Sleep until Start", time.Until(ls.Start)) // TODO: remove(?)
			// Sleep until Start
			time.Sleep(time.Until(ls.Start))
		default:
			log.Println("Monitoring light...") // TODO: remove(?)
			// Monitor light
			light := make(chan int, 2)
			quit := make(chan bool)
			go sendLight(ls.Pin, ls.Interval, ls.LightFactor, light, quit)
			for time.Now().After(ls.Start) && time.Now().Before(ls.Stop) {
				l := <-light
				log.Printf("Storing light %v...", l)
				ls.Data = shiftSlice(ls.Data, l)
				// TODO: store light into a log file (via go func?)
			}
			close(quit)
		}
	}
}

/*SendLight gathers light from pin every interval and send the light value
on to a channel. This loop runs until the quit chan is closed.*/
func sendLight(pin rpio.Pin, interval time.Duration, lightFactor int, light chan<- int, quit <-chan bool) {
	for {
		select {
		case _, _ = <-quit:
			log.Println("Closing monitorLight") // TODO: remove from log?
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

func shiftSlice(xi []int, x int) []int {
	for i := len(xi) - 1; i > 0; i-- {
		xi[i] = xi[i-1]
	}
	xi[0] = x
	return xi
}
