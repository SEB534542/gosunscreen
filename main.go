package main

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

// https://pkg.go.dev/github.com/stianeikeland/go-rpio?tab=doc

var sunrise = stoTime("10:00", 0)    // Time after which sunscreen can shine on the sunscreen area
var sunset = stoTime("23:00", 0)     // Time after which sunscreen no can shine on the sunscreen area
const sunsetThreshold int = 70       // minutes before sunset that sunscreen no longer should go down
const interval int = 60              // interval for checking current light in seconds
const lightGoodValue int = 9         // max measured light value that counts as "good weather"
const ligthGoodThreshold int = 15    // number of times light should be below lightGoodValue
const lightNeutralValue int = 9      // max measured light value that counts as "neutral weather"
const ligthNeutralThreshold int = 15 // number of times light should be above lightNeutralValue
const lightBadValue int = 9          // max measured light value that counts as "bad weather"
const ligthBadThreshold int = 15     // number of times light should be above lightBadValue
const allowedOutliers int = 2        // number of outliers accepted in the measurement

type sunscreen struct {
	secDown  int    // Seconds to move sunscreen down
	secUp    int    // Seconds to move sunscreen up
	position string // Position of sunscreen
	pinDown  int    // GPIO pin for moving sunscreen down
	pinUp    int    // GPIO pin for moving sunscreen up
}

type lightSensor struct {
	pinLight int   // pin for retrieving light value
	data     []int // collected light values
}

// Move suncreen up or down based on the sunscreen.position
func (s *sunscreen) move() {
	if s.position != "up" {
		log.Printf("Sunscreen position is %v, moving sunscreen up", s.position)
		// TODO: move sunscreen up
		s.position = "up"
	} else {
		log.Printf("Sunscreen position is %v, moving sunscreen down", s.position)
		// TODO: move sunscreen down
		s.position = "down"
	}
}

func (s *sunscreen) up() {
	if s.position != "up" {
		s.move()
	}
}

// Measure light from specified GPIO pin and return value
func (ls *lightSensor) getCurrentLight() int {
	// TODO: measure light
	return 5
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

	var counter int

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
	defer sunscreenMain.up()

	switch {
	case sunset.Sub(time.Now()).Minutes() <= float64(sunsetThreshold) && sunset.Sub(time.Now()).Minutes() > 0 && sunscreenMain.position == "up":
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

	ls1.data = append(ls1.data, ls1.getCurrentLight())
	if len(ls1.data) > 15 {
		ls1.data = ls1.data[(len(ls1.data) - 15):(len(ls1.data))]

		counter = 0
		switch sunscreenMain.position {
		case "up":
			log.Printf("Sunscreen is %v. Check if weather is good to go down\n", sunscreenMain.position)
			//for _, v := range ls1.data[]
		case "down":
			log.Printf("Sunscreen is %v.\n", sunscreenMain.position)
		}

		data := []int{2, 3, 4, 5, 6, 7, 8, 10}
		i := 3
		j := len(data)
		requiredValue := 5
		counter = 0
		for _, v := range data[i:j] {
			if v <= requiredValue {
				counter++
			}
		}
		fmt.Println(counter)
	}
	// def check_light(required_value, threshold):
	// result = False
	// count = 0
	// if sunscreen_status == 'up':
	//     for i in range(1, threshold+allowed_deviation+1):
	//         try:
	//             if light_list[i * -1] <= required_value:
	//                 count += 1
	//         except IndexError:
	//             # List is smaller than threshold
	//             pass
	// if sunscreen_status == 'down':
	//     for i in range(1, threshold+allowed_deviation+1):
	//         try:
	//             if light_list[i * -1] >= required_value:
	//                 count += 1
	//         except IndexError:
	//             # List is smaller than threshold
	//             pass
	// if count >= threshold:
	//     result = True
	// return result
	//TODO: add cases: sunscreen up / down vs weather
	//TODO: defer: GPIO clean-up + move sunscreen
	//TODO: add keyboard interrupt

}
