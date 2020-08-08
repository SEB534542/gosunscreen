package main

import (
	"fmt"
	"encoding/csv"
	"log"
	"os"
)

var data = [][]string{{"Line1", "Hello Readers of"}, {"Line2", "golangcode.com"}}

func main() {
    file, err := os.Open("result.csv")
    checkError("Cannot open file", err)
    defer file.Close()
	
	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

fmt.Print(records)

    writer := csv.NewWriter(file)
    defer writer.Flush()

    for _, value := range data {
        err := writer.Write(value)
        checkError("Cannot write to file", err)
    }
}

func checkError(message string, err error) {
    if err != nil {
        log.Fatal(message, err)
    }
}
