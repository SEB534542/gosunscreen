package main

import (
    "encoding/csv"
    "os"
    "log"
)

func main() {
	const file = "result.csv"
    // read the file
    f, err := os.Open(file)
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()
    r := csv.NewReader(f)
    lines, err := r.ReadAll()
    if err != nil {
        log.Fatal(err)
    }
    if err = f.Close(); err != nil {
       log.Fatal(err)
    }

    // add data
    var data = [][]string{{"Line1", "Hello Readers of"}}
    log.Printf("%T", lines)
    lines = append(lines, data)
 
    // write the file
    f, err = os.Create(file)
    if err != nil {
        log.Fatal(err)
    }
    w := csv.NewWriter(f)
    if err = w.WriteAll(lines); err != nil {
        log.Fatal(err)
        
    }
    
}
