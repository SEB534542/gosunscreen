package main

import (
	"fmt"
	"log"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

/* LightSensor represents a physical lightsensor for which data can be collected
through the corresponding GPIO pin.*/
type LightSensor struct {
	Pin          rpio.Pin      // pin for retrieving light value.
	Interval     time.Duration // Interval for checking current light in seconds.
	LightFactor  int           // Factor for correcting the measured analog light value.
	Start        time.Time     // Start time for measuring light.
	Stop         time.Time     // Stop time for measuring light.
	Good         int           // Max measured light value that counts as "good weather".
	TimesGood    int           // Number of times light should be below lightGoodValue.
	Neutral      int           // Max measured light value that counts as "neutral weather".
	TimesNeutral int           // Number of times light should be above lightNeutralValue.
	Bad          int           // max measured light value that counts as "bad weather".
	TimesBad     int           // number of times light should be above lightBadValue.
	Outliers     int           // Number of outliers accepted in the measurement.
	Data         []int         // collected light values.
}

const (
	maxCount                  = 9999999          // Maximum allowed count value while measuring light.
	freq                      = 10               // Number of times light is measured to get an average value.
	LightMin                  = 5                // Minimum value that can be stored for LightSensor.Good, Neutral or Bad.
	IntervalMin time.Duration = time.Second * 60 // Minimum seconds the interval should have
)

/* GetLight Takes a pin, measures the current light from the sensor on that rpio pin and
returns the value and error message.*/
func getLight(pin rpio.Pin) (int, error) {
	count := 0
	// Output on the pin for 0.1 seconds
	pin.Output()
	pin.Low()
	time.Sleep(100 * time.Millisecond)

	// Change the pin back to input
	pin.Input()
	// Count until the pin goes high
	for pin.Read() == rpio.Low {
		count++
		if count > maxCount {
			return count, fmt.Errorf("Count is getting too high (%v)", count)
		}
	}
	if count == 0 {
		return count, fmt.Errorf("Count is zero")
	}
	return count, nil
}

// TODO: ensure that output from GetCurrentLight() is divided by LightFactor in main code section.

/* GetCurrentLight takes a pin and frequency, collects the input from the light
sensor on that rpio pin and returns the average value as a slice of int together
with any errors. If int returned is zero, it means no light was measured
(which is accompanied with an error). However, it can be the case that some of
the attempts failed (ie errors generated), but a light value was measured.*/
func getAvgLight(pin rpio.Pin, freq int) (int, error) {
	values := []int{}
	var errs string
	i := 0
	for i < freq {
		value, err := getLight(pin)
		if err != nil {
			errs = fmt.Sprintf("%v\n\t%v", errs, err)
		}
		values = append(values, value)
		i++
	}
	x := calcAverage(values...)

	// Error handling
	var err error
	switch {
	case len(values) == 0:
		err = fmt.Errorf("All %v attempts failed. Errors:%v", freq, errs)
	case len(values) != freq || x == 0:
		err = fmt.Errorf("%v/%v attempts failed. %v Errors:%v", freq-len(values), freq, values, errs) // TODO: remove values
	}
	return x, err
}

/*CalAverage takes a slice of int and returns the average.
If the slice is empty, it will return zero as well.*/
func calcAverage(xi ...int) int {
	if len(xi) == 0 {
		return 0
	}
	total := 0
	for _, v := range xi {
		total = total + v
	}
	return total / len(xi)
}

func (ls *LightSensor) MonitorMove(s *Sunscreen) {
	for {
		switch {
		case time.Now().After(ls.Stop):
			log.Println("Reset Start and Stop for light monitoring to tomorrow") // TODO: remove(?)
			// Reset Start and Stop for both Sunscreen and Lightsensor to tomorrow
			updateStartStop(s, ls, 1)
			fallthrough
		case time.Now().Before(ls.Start):
			log.Printf("Sleep light monitoring for %v until %v", time.Until(ls.Start), ls.Start) // TODO: remove(?)
			// Sleep until Start
			time.Sleep(time.Until(ls.Start))
		default:
			log.Printf("Start monitoring light every %v", ls.Interval) // TODO: remove(?)
			// Monitor light
			light := make(chan int, 2)
			quit := make(chan bool)
			go sendLight(ls.Pin, ls.Interval, ls.LightFactor, light, quit)
			// Receive light
			for time.Now().After(ls.Start) && time.Now().Before(ls.Stop) {
				l := <-light
				log.Printf("Storing light %v...", l)
				maxL = max(ls.TimesGood, ls.TimesNeutral, ls.TimesBad) + ls.Outliers + 1
				ls.Data = addData(ls.Data, maxL, l)
				if s != nil {
					m := min(ls.TimesGood, ls.TimesNeutral, ls.TimesBad) + ls.Outliers
					// Only evaluatie sunscreen position if enough data has been gathered and mode == auto
					if len(ls.Data) >= m && s.Mode == auto {
						s.evaluate(ls.Data, ls.Good, ls.Neutral, ls.Bad, ls.TimesGood, ls.TimesNeutral, ls.TimesBad, ls.Outliers)
						// TODO: store light into a log file (via go func?)
					}
				}
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

func addData(xi []int, maxL, x int) []int {
	if len(xi) < maxL {
		xi = append(xi, x)
		return xi
	}
	xi = shiftSlice(xi, x)
	if len(xi) > maxL {
		xi = xi[len(xi)-maxL:]
	}
	return xi
}

func shiftSlice(xi []int, x int) []int {
	for i := len(xi) - 1; i > 0; i-- {
		xi[i] = xi[i-1]
	}
	xi[0] = x
	return xi
}

func (ls *LightSensor) reset() {
	// TODO based on outliers + max times
}
