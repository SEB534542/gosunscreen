package main

import (
	"fmt"

	"github.com/stianeikeland/go-rpio/v4"
)

/* GetLight Takes a pin, measures the current light from the sensor on that rpio pin and
returns the value and error message.*/
func getLight(pin rpio.Pin) (int, error) {
	count := 0
	// Output on the pin for 0.1 seconds
	pin.Output()
	pin.PinLight.Low()
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
		return count, fmt.Errorf("Count is zero (%v)", count)
	}
	return count, nil
}

// TODO: ensure that output from GetCurrentLight() is divided by ls.LightFactor

/* GetCurrentLight takes a pin and frequency, collects the input from the light
sensor on that rpio pin and returns the average value as a slice of int together
with any errors. */
func getAvgLight(pin rpio.Pin, freq int) (int, error) {
	values := make([]int, freq, freq)
	var errs string
	i := 0
	for i < len(values) {
		value, err := getLight(pin)
		if err != nil {
			errs = fmt.Sprintf("%v\n\t%v", errs, err)
			// Value should not be stored as there is an error, so shorten slice
			values = append(values[:i], values[i+1:]...)
		}
		values[i] = value
		i++
	}
	x := calcAverage(values...)

	// Error handling
	err = nil
	switch {
	case len(values) == 0:
		err = fmt.Errorf("All of the %v attempts failed from pin %v. Errors:%v", freq, pin, errs)
	case x == 0:
		// Average is zero
		err = fmt.Errorf("Average light from pin %v is zero after %v attempts. Errors:%v", pin, freq, errs)
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

// func main() {
// 	factor := 50
// 	for {
// 		if h := time.Now().Hour(); h > 8 && h < 22 {
// 			value, err := getAvgLight(rpio.Pin(5))
// 			if err != nil {
// 				log.Println(err)
// 			} else {
// 				log.Println("Light gathered:", value/factor)
// 			}
// 			time.Sleep(60 * time.Second)
// 		}
// 	}
// }
