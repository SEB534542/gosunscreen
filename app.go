package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio/v4"
)

// Constants for config folder and files
const (
	folderConfig    = "config"
	fileConfig      = "./config/config.json"
	fileSunscrn     = "./config/sunscreen.json"
	fileLightsensor = "./config/lightsensor.json"
	folderLog       = "logs"
	fileLog         = "./logs/logfile.log"
	fileStats       = "./logs/sunscreen_stats.csv"
	fileLight       = "./logs/light_stats.csv"
)

var (
	ls     = &LightSensor{}
	s      = &Sunscreen{}
	config Config
)

var (
	muSunscrn sync.Mutex
	muLS      sync.Mutex
	muConf    sync.Mutex
)

func init() {
	// Check if log folder exists, else create
	if _, err := os.Stat(folderLog); os.IsNotExist(err) {
		os.Mkdir(folderLog, 4096)
	}
	// Check if log folder exists, else create
	if _, err := os.Stat(folderConfig); os.IsNotExist(err) {
		os.Mkdir(folderConfig, 4096)
	}
}

// TODO: review global/local funcs and vars

func main() {
	// TODO: check if below can be stored in a separate func
	// Open logfile or create if not exists
	f, err := os.OpenFile(fileLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Panic("Error opening log file:", err)
		//  TODO: remove log file and create new
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println("--------Start of program--------")

	// Open connection RPIO pins
	rpio.Open()
	loadConfig()
	s.init()
	updateStartStop(s, ls, 0)

	log.Println("Starting monitor")
	if ls != nil {
		go ls.MonitorMove(s)
	}
	startServer()
}

// UpdateStartStop resets all start/stop to today + d (e.g. d=0 resets it to today.
func updateStartStop(s *Sunscreen, ls *LightSensor, d int) {
	s.resetStartStop(d)
	fmt.Println("done resetss sunscreen")
	// Light sensor should start in time so at sunscreen start enough light has been gathered
	muLS.Lock()
	dur := time.Duration((max(ls.TimesGood, ls.TimesNeutral, ls.TimesBad)+ls.Outliers)/int(ls.Interval.Minutes())) * time.Minute
	ls.Start = s.Start.Add(-dur)
	ls.Stop = s.Stop.Add(time.Duration(30 * time.Minute))
	muLS.Unlock()
}

// Max takes multiple int and returns the highest value. It always returns a minimum of zero.
func max(xi ...int) int {
	var x int
	for _, v := range xi {
		if v > x {
			x = v
		}
	}
	return x
}

// Min takes multiple int and returns the highest value. It always returns a minimum of 100000000.
func min(xi ...int) int {
	x := 100000000
	for _, v := range xi {
		if v < x {
			x = v
		}
	}
	return x
}

// SendMail sends mail to
func sendMail(subj, body string) {
	if config.EnableMail {
		//Format message
		var msgTo string
		for i, s := range config.MailTo {
			if i != 0 {
				msgTo = msgTo + ","
			}
			msgTo = msgTo + s
		}

		msg := []byte("To:" + msgTo + "\r\n" +
			"Subject:" + subj + "\r\n" +
			"\r\n" + body + "\r\n")

		// Set up authentication information
		auth := smtp.PlainAuth("", config.MailUser, config.MailPass, config.MailHost)

		// Connect to the server, authenticate, set the sender and recipient,
		// and send the email all in one step.
		err := smtp.SendMail(fmt.Sprintf("%v:%v", config.MailHost, config.MailPort), auth, config.MailFrom, config.MailTo, msg)
		if err != nil {
			log.Println("Unable to send mail:", err)
			return
		}
		log.Println("Send mail to", config.MailTo)
	}
}
