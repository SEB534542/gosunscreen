package gosunscreen

import (
	"testing"
	"time"
)

func TestSetTime(t *testing.T) {
	type test struct {
		dataTime  string
		dataDelta int
		expected  time.Time
	}

	tests := []test{
		{"10:00", 1, time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day()+1, 10, 0, 0, 0, time.Local)},
		{"12:30", 0, time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 12, 30, 0, 0, time.Local)},
		{"13:21", -1, time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day()-1, 13, 21, 0, 0, time.Local)},
		{"13:01", -1, time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day()-1, 13, 1, 0, 0, time.Local)},
		{"09:01", -1, time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day()-1, 9, 1, 0, 0, time.Local)},
		{"00:00", 0, time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)},
	}

	for _, v := range tests {
		x := stoTime(v.dataTime, v.dataDelta)

		if x != v.expected {
			t.Error("Expected", v.expected, "Got", x)
		}
	}
}

func TestMove(t *testing.T) {

	type test struct {
		position string
		expected string
	}

	tests := []test{
		{"up", "down"},
		{"down", "up"},
		{"hallo", "up"},
	}

	for _, v := range tests {
		s := &sunscreen{
			secDown:  17,
			secUp:    20,
			position: v.position,
			pinDown:  40,
			pinUp:    38,
		}
		s.move()
		x := v.expected
		if s.position != x {
			t.Error("Expected", v.expected, "Got", x)
		}
	}
}

func TestautoSunscreen(){
	config : = struct {
		Sunrise               time.Time // Time after which Sunscreen can shine on the Sunscreen area
		Sunset                time.Time // Time after which Sunscreen no can shine on the Sunscreen area
		SunsetThreshold       int       // Minutes before sunset that Sunscreen no longer should go down
		Interval              int       // Interval for checking current light in seconds
		LightGoodValue        int       // Max measured light value that counts as "good weather"
		LightGoodThreshold    int       // Number of times light should be below lightGoodValue
		LightNeutralValue     int       // Max measured light value that counts as "neutral weather"
		LightNeutralThreshold int       // Number of times light should be above lightNeutralValue
		LightBadValue         int       // max measured light value that counts as "bad weather"
		LightBadThreshold     int       // number of times light should be above lightBadValue
		AllowedOutliers       int       // Number of outliers accepted in the measurement
		RefreshRate           int       // Number of seconds the main page should refresh
		EnableMail            bool      // Enable mail functionality
		MoveHistory           int       // Number of sunscreen movements to be shown
	}{
		Sunrise: time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 9, 0, time.Now().Second(), time.Now().Nanosecond(), time.Local)
		Sunset: time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 0, time.Now().Second(), time.Now().Nanosecond(), time.Local)
		SunsetThreshold: 70
		Interval: 1
		LightGoodValue: 9
		LightGoodThreshold: 3
		LightNeutralValue: 11
		LightNeutralThreshold: 3
		LightBadValue: 28
		LightBadThreshold: 3
		AllowedOutliers: 2
		RefreshRate: 10
		EnableMail: false
		MoveHistory: 0
	}
}
