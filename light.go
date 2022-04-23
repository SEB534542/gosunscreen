package main

import (
	"fmt"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

const (
	maxCount = 9999999 // Maximum allowed count value while measuring light
	freq     = 10      // Number of times light is measured to get an average value
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
	case len(values) != freq:
		err = fmt.Errorf("%v/%v attempts failed. %v Errors:%v", freq-len(values), freq, values, err) // TODO: remove values
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
