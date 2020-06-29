package main

import (
	"testing"
	"time"
)

func TestSetTime(t *testing.T) {
	type test struct {
		dataTime  string
		dataDelta int
		answer    time.Time
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

		if x != v.answer {
			t.Error("Expected", v.answer, "Got", x)
		}
	}
}
