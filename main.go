package main

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

type sunscreen struct {
	sunrise  time.Time // Time after which sunscreen can shine on the sunscreen area
	sunset   time.Time // Time after which sunscreen no can shine on the sunscreen area
	secDown  int       // Seconds to move sunscreen down
	secUp    int       // Seconds to move sunscreen up
	position string    // Position of sunscreen
	pinDown  int       // GPIO pin for moving sunscreen down
	pinUp    int       // GPIO pin for moving sunscreen up
}

const sunsetThreshold int = 70 // minutes before sunset that sunscreen no longer should go down
const interval int = 60        // interval for checking current light in seconds

const lightGoodValue int = 9      // max measured light value that counts as "good weather"
const ligthGoodThreshold int = 15 // number of times light should be below lightGoodValue

const lightNeutralValue int = 9      // max measured light value that counts as "neutral weather"
const ligthNeutralThreshold int = 15 // number of times light should be above lightNeutralValue

const lightBadValue int = 9      // max measured light value that counts as "bad weather"
const ligthBadThreshold int = 15 // number of times light should be above lightBadValue

const allowedOutliers int = 2 // number of outliers accepted in the measurement

const pinSunlight int = 16 // pin for retrieving light value

var sunscreenStatus = "unknown"

func main() {
	s1 := sunscreen{
		sunrise:  stoTime("10:00", 0),
		sunset:   stoTime("23:00", 0),
		secDown:  17,
		secUp:    20,
		position: "unknown",
		pinDown:  40,
		pinUp:    38,
	}
	if sunscreenStatus == "unknown" {
		moveSunscreen("up")
	}

	light := []int{}

	switch {
	case s1.sunset.Sub(time.Now()) < 0:
		log.Printf("Sun is down (%v), adjusting sunrise/set to tomorrow", s1.sunset)
		//TODO: adjust time by one day
		fallthrough
	case s1.sunrise.Sub(time.Now()) > 0:
		log.Printf("Sun is not yet up (%v), snoozing", s1.sunrise)
		// sleep until sunrise
		fallthrough
	default:
		fmt.Println("Sunrise:", s1.sunrise)
		fmt.Println("Sunset:", s1.sunset)
		light = append(light, getLight(pinSunlight))
		fmt.Println("Light is:", light)
	}

	//TODO: configure GPIO
	//TODO: init: move sunscreen (up)
	//TODO: add logic: x minutes before sunset
	//TODO: add cases: sunscreen up / down vs weather
	//TODO: defer: GPIO clean-up + move sunscreen
	//TODO: add keyboard interrupt

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

// Measure light from specified GPIO pin and return value
func getLight(pin int) int {
	// TODO: measure ligth
	fmt.Println("Light pin:", pin)
	return 5
}

//
func moveSunscreen(direction string) {
	// Include error if direction is not up or down
}

func close() {
	if sunscreenStatus != "up" {
		moveSunscreen("up")
	}
}
