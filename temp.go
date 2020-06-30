package main

import "fmt"

type person struct {
	first string
}

func (p person) changeName() {
	p.first = "Person2"
	fmt.Println(p.first)
}

func main() {
	p1 := person{"Person1"}
	p1.changeName()
	fmt.Println(p1.first)
}

// 	const sunriseTime string = "1s:f0"
// 	t := sunriseTime
// 	i, err := strconv.ParseInt(t[3:], 0, 0)
// 	if err != nil {
// 		log.Panicf("Time %s is not correctly formatted. Please unsure time is written as hh:mm", t)
// 	}
// 	fmt.Println(i)
// }

//funct timetest() {
// tNow := time.Now()
// fmt.Println(tNow)
// fmt.Println(tNow.Local())
// t := time.Date(2020, time.June, 29, 23, 0, 0, 0, time.Local)
// fmt.Println(t)
// difference := t.Sub(tNow)
// fmt.Println("Difference =", difference)
//}

// func switchtest() {
// 	//tNow := time.Now()
// 	sunrise := time.Date(2020, time.June, 28, 10, 0, 0, 0, time.Local)
// 	sunset := time.Date(2020, time.June, 28, 22, 0, 0, 0, time.Local)

// 	fmt.Println("time to sunset", sunset.Sub(time.Now()))
// 	fmt.Println("time to sunrise", sunrise.Sub(time.Now()))

// 	if sunrise.Sub(time.Now()) < 0 {
// 		fmt.Println("smaller than zero")
// 	}

// 	switch {
// 	case sunrise.Sub(time.Now()) < 0:
// 		fmt.Println("Sun is up")
// 		fallthrough
// 	case sunset.Sub(time.Now()) > 0:
// 		fmt.Println("Sun is not yet under")
// 	}
// }
