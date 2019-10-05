package main

import (
	"log"
	"os"
	"net/mail"
	"net/smtp"
	"io"
	"io/ioutil"
	"time"
	"bytes"
	"fmt"
)

type Message struct {
	Subject     string
	From        string
	To          string
	Cc          string
	Bcc         string
	Date        string
	Id          string
	InReplyTo   string
	ContentType string
	XList       string
	Body        string
}


// Read a message from the given io.Reader
func (msg *Message) FromReader(stream io.Reader) error {
	inMessage, err := mail.ReadMessage(stream)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(inMessage.Body)
	if err != nil {
		return err
	}

	msg.Subject = inMessage.Header.Get("Subject")
	msg.From = inMessage.Header.Get("From")
	msg.Id = inMessage.Header.Get("Message-ID")
	msg.InReplyTo = inMessage.Header.Get("In-Reply-To")
	msg.Body = string(body[:])
	msg.To = inMessage.Header.Get("To")
	msg.Cc = inMessage.Header.Get("Cc")
	msg.Bcc = inMessage.Header.Get("Bcc")
	msg.Date = inMessage.Header.Get("Date")

	return nil
}

// Create a new message that replies to this message
func (msg *Message) Reply() *Message {
	reply := &Message{}
	reply.Subject = "Re: " + msg.Subject
	reply.To = msg.From
	reply.InReplyTo = msg.Id
	reply.Date = time.Now().Format("Mon, 2 Jan 2006 15:04:05 -0700")
	return reply
}

// Prepare a copy of the message that we're forwarding to a list
func (msg *Message) ResendAs(listId string, listAddress string) *Message {
	send := &Message{}
	send.Subject = msg.Subject
	send.From = msg.From
	send.To = msg.To
	send.Cc = msg.Cc
	send.Date = msg.Date
	send.Id = msg.Id
	send.InReplyTo = msg.InReplyTo
	send.XList = listId + " <" + listAddress + ">"

	// If the destination mailing list is in the Bcc field, keep it there
	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			if bcc.Address == listAddress {
				send.Bcc = listId + " <" + listAddress + ">"
				break
			}
		}
	}
	return send
}

// Generate a emailable represenation of this message
func (msg *Message) String() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "From: %s\r\n", msg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	fmt.Fprintf(&buf, "Cc: %s\r\n", msg.Cc)
	fmt.Fprintf(&buf, "Bcc: %s\r\n", msg.Bcc)
	if len(msg.Date) > 0 {
		fmt.Fprintf(&buf, "Date: %s\r\n", msg.Date)
	}
	if len(msg.Id) > 0 {
		fmt.Fprintf(&buf, "Messsage-ID: %s\r\n", msg.Id)
	}
	fmt.Fprintf(&buf, "In-Reply-To: %s\r\n", msg.InReplyTo)
	if len(msg.XList) > 0 {
		fmt.Fprintf(&buf, "X-Mailing-List: %s\r\n", msg.XList)
		fmt.Fprintf(&buf, "List-ID: %s\r\n", msg.XList)
		fmt.Fprintf(&buf, "Sender: %s\r\n", msg.XList)
	}
	if len(msg.ContentType) > 0 {
		fmt.Fprintf(&buf, "Content-Type: %s\r\n", msg.ContentType)
	}
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "\r\n%s", msg.Body)

	return buf.String()
}

func (msg *Message) Send(recipients []string) {
	if gConfig.Debug {
		fmt.Printf("------------------------------------------------------------\n")
		fmt.Printf("SENDING MESSAGE TO:\n")
		for _, r := range recipients {
			fmt.Printf(" - %s\n", r)
		}
		fmt.Printf("MESSAGE:\n")
		fmt.Printf("%s\n", msg.String())
		return
	}

	auth := smtp.PlainAuth("", gConfig.SMTPUsername, gConfig.SMTPPassword, gConfig.SMTPHostname)
	err := smtp.SendMail(gConfig.SMTPHostname+":"+gConfig.SMTPPort, auth, msg.From, recipients, []byte(msg.String()))
	if err != nil {
		log.Printf("EROR_SENDING Error=%q\n", err.Error())
		os.Exit(0)
	}
}
