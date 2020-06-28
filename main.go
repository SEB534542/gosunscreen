package main

import (
	"fmt"
	"time"
)

const sunriseTime string = "10:00" // prefered sunrise time in "hh:mm"
const sunsetTime string = "18:15"  // prefered sunset time in "hh:mm"
const sunscreenDown int = 17       // seconds
const sunscreenUp int = 20         // seconds
const sunsetThreshold int = 70     // minutes before sunset that sunscreen no longer should go down
const interval int = 60            // interval for checking current light in seconds

const lightGoodValue int = 9      // max measured light value that counts as "good weather"
const ligthGoodThreshold int = 15 // number of times light should be below lightGoodValue

const lightNeutralValue int = 9      // max measured light value that counts as "neutral weather"
const ligthNeutralThreshold int = 15 // number of times light should be above lightNeutralValue

const lightBadValue int = 9      // max measured light value that counts as "bad weather"
const ligthBadThreshold int = 15 // number of times light should be above lightBadValue

const allowedOutliers int = 2 // number of outliers accepted in the measurement

const pinSuncreenUp int = 38    // pin for moving sunscreen up
const pinSunscreenDown int = 40 // pin for moving sunscreen down
const pinSunlight int = 16      // pin for retrieving light value

func main() {

	sunrise := setTime(sunriseTime, 1)
	sunset := setTime(sunsetTime, 0)

	fmt.Println(sunrise)
	fmt.Println(sunset)

	//TODO: Set vars using func init()
	//TODO: configure GPIO
	//TODO: init: move sunscreen (up)
	//TODO: add cases: time < sunrise / time > sunset / x minutes before sunset
	//TODO: measure light
	//TODO: add cases: sunscreen up / down vs weather
	//TODO: defer: GPIO clean-up + move sunscreen
	//TODO: add keyboard interrupt

}

// Based on the received string time (format hh:mm) and the offset, the func returns a type time with today's today + the offset
func setTime(t string, d int) time.Time {
	timeNow := time.Now().Day()
	return time.Date(timeNow.Year, timeNow.Month, timeNow.Day+d, int(t[:2]), int(sunriseTime), 0, 0, time.Local)
}
