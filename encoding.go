package main

import (
	"encoding/json"
	"io/ioutil"
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
		pw, err := bcrypt.GenerateFromPassword([]byte("today"), bcrypt.MinCost)
		if err != nil {
			log.Fatal("Error setting default password:", err)
		}
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
