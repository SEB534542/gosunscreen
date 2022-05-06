package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// SaveToJson takes an interface and stores it into the filename
func SaveToJSON(i interface{}, fileName string) {
	bs, err := json.Marshal(i)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(fileName, bs, 0644)
	if err != nil {
		log.Fatal("Error", err)
	}
}

// ReadJSON reads from the given json file location and returns any error.
// into i interface.
func readJSON(fname string, i interface{}) error {
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		log.Printf("File '%v' does not exist, creating blank", fname)
		SaveToJSON(i, fname)
	} else {
		data, err := ioutil.ReadFile(fname)
		// TODO: remove file and create new
		if err != nil {
			return fmt.Errorf("%s is corrupt. Please delete the file (%v)", fname, err)
		}
		err = json.Unmarshal(data, i)
		if err != nil {
			return fmt.Errorf("%s is corrupt. Please delete the file (%v)", fname, err)
		}
	}
	return nil
}

// LoadConfig reads the JSON file from fname and does some initial checks.
func loadConfig() {
	// Load config
	err := readJSON(fileConfig, &config)
	if err != nil {
		log.Printf("Error while reading JSON '%v', please manually set config and save", fileConfig)
	}
	if config.Port == 0 {
		config.Port = 8081
		log.Printf("Unable to load port, set port to default (%v)", config.Port)
	}
	if config.Username == "" {
		config.Username = "admin"
		pw := []uint8{36, 50, 97, 36, 48, 52, 36, 71, 89, 66, 56, 116, 79, 102, 65, 57, 52, 84, 114, 82, 46, 107, 89, 65, 65, 71, 73, 77, 79, 76, 108, 81, 69, 114, 99, 68, 104, 52, 88, 81, 79, 89, 115, 81, 78, 99, 69, 53, 53, 73, 73, 97, 73, 114, 71, 70, 50, 81, 103, 46}
		config.Password = pw
	}
	if config.RefreshRate == time.Duration(0) {
		config.RefreshRate, err = time.ParseDuration("1h")
		log.Fatal("Error setting default refreshrate:", err)
	}

	// Load sunscreen
	err = readJSON(fileSunscrn, &s)
	if err != nil {
		log.Fatal(err)
	}

	// Load lightsensor
	err = readJSON(fileLightsensor, &ls)
	if err != nil {
		log.Fatal(err)
	}
	ls.Data = []int{} // Make sure data is empty (since restarted)
	// TODO: ensure that if ls is stored (saveJSON), data is stored empty(?)
}
