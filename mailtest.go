package main

import (
	"net/smtp"
	"log"
)


//const mailAdrList = []string{"shj.vandermeulen@gmail.com"}


func main() {
	sendMail() 
}

// SendMail sends mail
func sendMail() {
	// Set up authentication information.
	auth := smtp.PlainAuth("", "raspberrych57@gmail.com", "Raspberrych4851", "smtp.gmail.com")

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	to := []string{"shj.vandermeulen@gmail.com"}
	msg := []byte("To: shj.vandermeulen@gmail.com\r\n" +
		"Subject: discount Gophers!\r\n" +
		"\r\n" +
		"This is the email body.\r\n")
	err := smtp.SendMail("smtp.gmail.com:587", auth, "raspberrych57@gmail.com", to, msg)
	if err != nil {
		log.Fatal(err)
	}
}
