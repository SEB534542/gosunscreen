package main

import (
	"fmt"
	"time"

	"github.com/SEB534542/seb"
	"github.com/kelvins/sunrisesunset"
)

type Config struct {
	Location sunrisesunset.Parameters // Contains Latiude, longitude, UtcOffset and Date
	Sunrise  time.Time                // Date and time of sunrise for Location
	Sunset   time.Time                // Date and time of sunset for Location
}

func main() {
	config := Config{
		Location: sunrisesunset.Parameters{
			Latitude:  52.3730840,
			Longitude: 4.899023,
			UtcOffset: 1.0,
			Date:      time.Now(),
		},
	}
	var err error
	config.Sunrise, config.Sunset, err = config.Location.GetSunriseSunset()

	fmt.Println(config, err)
	seb.SaveToJSON(config, "test.json")
}
