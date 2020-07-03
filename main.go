// Package gosunscreen monitors light and moves the sunscreen accordingly through GPIO
package main

import (
	"log"
	"strconv"
	"time"
)

// https://pkg.go.dev/github.com/stianeikeland/go-rpio/v4?tab=doc
// import "github.com/stianeikeland/go-rpio/v4"

var sunrise = stoTime("10:00", 0)              // Time after which sunscreen can shine on the sunscreen area
var sunset = stoTime("23:00", 0)               // Time after which sunscreen no can shine on the sunscreen area
const sunsetThreshold int = 70                 // minutes before sunset that sunscreen no longer should go down
const interval time.Duration = 5 * time.Second // interval for checking current light in seconds
const lightGoodValue int = 9                   // max measured light value that counts as "good weather"
const ligthGoodThreshold int = 15              // number of times light should be below lightGoodValue
const lightNeutralValue int = 9                // max measured light value that counts as "neutral weather"
const ligthNeutralThreshold int = 15           // number of times light should be above lightNeutralValue
const lightBadValue int = 9                    // max measured light value that counts as "bad weather"
const ligthBadThreshold int = 15               // number of times light should be above lightBadValue
const allowedOutliers int = 2                  // number of outliers accepted in the measurement

const up string = "up"
const down string = "down"

// A Sunscreen represents a physical sunscreen that can be controlled through 2 GPIO pins: one for moving it up, and one for moving it down.
type sunscreen struct {
	secDown  int    // Seconds to move sunscreen down
	secUp    int    // Seconds to move sunscreen up
	position string // Current position of sunscreen
	pinDown  int    // GPIO pin for moving sunscreen down
	pinUp    int    // GPIO pin for moving sunscreen up
}

// A LightSensor represents a physical lightsensor which data can be collected through a GPIO pin.
type lightSensor struct {
	pinLight int   // pin for retrieving light value
	data     []int // collected light values
}

// Move moves the suncreen up or down based on the sunscreen.position. It updates the position accordingly.
func (s *sunscreen) move() {
	if s.position != up {
		log.Printf("Sunscreen position is %v, moving sunscreen up", s.position)
		// TODO: move sunscreen up
		s.position = up
	} else {
		log.Printf("Sunscreen position is %v, moving sunscreen down", s.position)
		// TODO: move sunscreen down
		s.position = down
	}
}

// Up checks if the suncreen's position is up. If not, it moves the suncreen up through method move().
func (s *sunscreen) up() {
	if s.position != up {
		s.move()
	}
}

// ReviewPosition reviews the position of the sunscreen against the light data and moves the sunscreen up or down if it meets the criteria
func (s *sunscreen) reviewPosition(lightData []int) {
	counter := 0
	switch s.position {
	case up:
		log.Printf("Sunscreen is %v. Check if weather is good to go down\n", s.position)
		for _, v := range lightData[:(ligthGoodThreshold + allowedOutliers)] {
			if v <= lightGoodValue {
				counter++
			}
		}
		if counter >= ligthGoodThreshold {
			s.move()
			return
		}
	case down:
		log.Printf("Sunscreen is %v. Check if it should go up\n", s.position)

		for _, v := range lightData[:(ligthNeutralThreshold + allowedOutliers)] {
			if v >= lightNeutralValue {
				counter++
			}
		}
		if counter >= ligthNeutralThreshold {
			s.move()
			return
		}

		for _, v := range lightData[:(ligthBadThreshold + allowedOutliers)] {
			if v >= lightBadValue {
				counter++
			}
		}
		if counter >= ligthBadThreshold {
			s.move()
			return
		}
	}
}

// GetCurrentLight collects the input from the light sensor ls and returns the value as a slice of int
func (ls *lightSensor) getCurrentLight() []int {
	// TODO: measure light
	return []int{5}
}

// MaxIntSlice receives variadic parameter of integers and return the highest integer
func maxIntSlice(xi ...int) int {
	var max int
	for i, v := range xi {
		if i == 0 || v < max {
			max = v
		}
	}
	return max
}

// Based on the received string time (format hh:mm) and the day offset, the func returns a type time with today's date + the offset in days
func stoTime(t string, days int) time.Time {
	timeNow := time.Now()

	timeHour, err := strconv.Atoi(t[:2])
	if err != nil {
		log.Panicf("Time %v is not correctly formatted. Please unsure time is written as hh:mm. Error: %v", t, err)
	}

	timeMinute, err := strconv.Atoi(t[3:])
	if err != nil {
		log.Panicf("Time %s is not correctly formatted. Please unsure time is written as hh:mm", t)
	}

	return time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day()+days, int(timeHour), int(timeMinute), 0, 0, time.Local)
}

func main() {
	log.Println("--------Start of program--------")

	ls1 := &lightSensor{
		pinLight: 16,
		data:     []int{},
	}

	sunscreenMain := &sunscreen{
		secDown:  17,
		secUp:    20,
		position: "unknown",
		pinDown:  40,
		pinUp:    38,
	}
	sunscreenMain.move()
	defer func() {
		log.Println("Closing down...")
		sunscreenMain.up()
	}()

	for {
		switch {
		case sunset.Sub(time.Now()).Minutes() <= float64(sunsetThreshold) && sunset.Sub(time.Now()).Minutes() > 0 && sunscreenMain.position == up:
			log.Printf("Sun will set in (less then) %v min and sunscreen is %v. Snoozing until sunset\n", sunsetThreshold, sunscreenMain.position)
			// TODO: Snooze until sunset
		case sunset.Sub(time.Now()) <= 0:
			log.Printf("Sun is down (%v), adjusting sunrise/set to tomorrow", sunset)
			sunscreenMain.up()
			ls1.data = []int{}
			sunrise = sunrise.AddDate(0, 0, 1)
			sunset = sunset.AddDate(0, 0, 1)
			fallthrough
		case sunrise.Sub(time.Now()) > 0:
			log.Printf("Sun is not yet up, snoozing until %v", sunrise)
			sunscreenMain.up()
		}

		ls1.data = append(ls1.getCurrentLight(), ls1.data...)

		if len(ls1.data) > maxIntSlice(lightGoodValue, lightBadValue, lightNeutralValue) {
			ls1.data = ls1.data[:15]
			sunscreenMain.reviewPosition(ls1.data)
		}

		log.Println("Completed cycle, sleeping...")
		time.Sleep(interval)
	}
}
