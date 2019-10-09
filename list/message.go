package list

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// Message represents an e-mail message
type Message struct {
	Subject         string
	From            string
	To              string
	Cc              string
	Bcc             string
	Date            string
	ID              string
	InReplyTo       string
	MIMEVersion     string
	ContentType     string
	XList           string
	ListUnsubscribe string
	Headers         map[string][]string
	Body            []byte
}

// FromReader reads a message from the given io.Reader
func (msg *Message) FromReader(stream io.Reader) error {
	inMessage, err := mail.ReadMessage(stream)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(inMessage.Body)
	if err != nil {
		return err
	}

	header := textproto.MIMEHeader(inMessage.Header)
	msg.Subject = header.Get("Subject")
	msg.From = header.Get("From")
	msg.ID = header.Get("Message-ID")
	msg.InReplyTo = header.Get("In-Reply-To")
	msg.Body = body
	msg.To = header.Get("To")
	msg.Cc = header.Get("Cc")
	msg.Bcc = header.Get("Bcc")
	msg.Date = header.Get("Date")
	msg.MIMEVersion = header.Get("MIME-Version")
	msg.ContentType = header.Get("Content-Type")

	header.Del("Subject")
	header.Del("From")
	header.Del("Message-ID")
	header.Del("In-Reply-To")
	header.Del("To")
	header.Del("Cc")
	header.Del("Bcc")
	header.Del("Date")
	header.Del("MIME-Version")
	header.Del("Content-Type")

	msg.Headers = map[string][]string(header)

	return nil
}

// Reply creates a new message that replies to the given message
func (msg *Message) Reply() *Message {
	reply := &Message{}
	reply.Subject = "Re: " + msg.Subject
	reply.To = msg.From
	reply.InReplyTo = msg.ID
	reply.Date = time.Now().Format("Mon, 2 Jan 2006 15:04:05 -0700")
	reply.MIMEVersion = "1.0"
	reply.ContentType = "text/plain; charset=utf-8"
	reply.Headers = map[string][]string{}
	reply.Body = []byte{}
	return reply
}

// ResendAs a list prepares a copy of the message to be used for a list forward
func (msg *Message) ResendAs(list *List, commandAddress string) *Message {
	send := &Message{}
	send.Subject = msg.Subject
	send.From = msg.From
	send.To = msg.To
	send.Cc = msg.Cc
	send.Date = msg.Date
	send.ID = msg.ID
	send.InReplyTo = msg.InReplyTo
	send.XList = fmt.Sprintf("%s <%s>", list.Name, list.ID)
	if !list.Locked {
		send.ListUnsubscribe = fmt.Sprintf("<mailto:%s?subject=unsubscribe>", commandAddress)
	}

	// If the destination mailing list is in the Bcc field, keep it there
	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			if bcc.Address == list.ID {
				send.Bcc = list.Name + " <" + list.ID + ">"
				break
			}
		}
	}
	return send
}

// String representing the message
func (msg *Message) String() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "From: %s\r\n", msg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", msg.To)
	if len(msg.Cc) > 0 {
		fmt.Fprintf(&buf, "Cc: %s\r\n", msg.Cc)
	}
	if len(msg.Bcc) > 0 {
		fmt.Fprintf(&buf, "Bcc: %s\r\n", msg.Bcc)
	}
	if len(msg.Date) > 0 {
		fmt.Fprintf(&buf, "Date: %s\r\n", msg.Date)
	}
	if len(msg.ID) > 0 {
		fmt.Fprintf(&buf, "Messsage-ID: %s\r\n", msg.ID)
	}
	if len(msg.InReplyTo) > 0 {
		fmt.Fprintf(&buf, "In-Reply-To: %s\r\n", msg.InReplyTo)
	}
	if len(msg.XList) > 0 {
		fmt.Fprintf(&buf, "X-Mailing-List: %s\r\n", msg.XList)
		fmt.Fprintf(&buf, "List-ID: %s\r\n", msg.XList)
		fmt.Fprintf(&buf, "Sender: %s\r\n", msg.XList)
	}
	if len(msg.ListUnsubscribe) > 0 {
		fmt.Fprintf(&buf, "List-Unsubscribe: %s\r\n", msg.ListUnsubscribe)
	}
	for key, values := range msg.Headers {
		for _, value := range values {
			fmt.Fprintf(&buf, "%s: %s\r\n", key, value)
		}
	}
	if len(msg.MIMEVersion) > 0 {
		fmt.Fprintf(&buf, "MIME-Version: %s\r\n", msg.MIMEVersion)
	}
	if len(msg.ContentType) > 0 {
		fmt.Fprintf(&buf, "Content-Type: %s\r\n", msg.ContentType)
	}
	fmt.Fprintf(&buf, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&buf, "\r\n")

	buf.Write(msg.Body)

	return buf.String()
}

// SendVERP sends a Message using an VARP
func (msg *Message) SendVERP(envelopeSender string, recipients []string, SMTPHostname string, SMTPPort uint64, SMTPUsername string, SMTPPassword string, debug bool) []error {
	parts := strings.SplitN(envelopeSender, "@", 2)
	if len(parts) < 2 {
		return []error{fmt.Errorf("Invalid envelope sender %s", envelopeSender)}
	}

	errors := []error{}
	for _, recipient := range recipients {
		envelope := fmt.Sprintf("%s+%s@%s", parts[0], strings.Replace(recipient, "@", "=", 1), parts[1])
		err := msg.Send(envelope, []string{recipient}, SMTPHostname, SMTPPort, SMTPUsername, SMTPPassword, debug)
		// Try others too
		if err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// Send a Message
func (msg *Message) Send(envelopeSender string, recipients []string, SMTPHostname string, SMTPPort uint64, SMTPUsername string, SMTPPassword string, debug bool) error {
	if debug {
		log.Print(msg.SendDebug(envelopeSender, recipients))
		return nil
	}
	var auth smtp.Auth
	if SMTPUsername != "" {
		auth = smtp.PlainAuth("", SMTPUsername, SMTPPassword, SMTPHostname)
	}
	return SendMail(fmt.Sprintf("%s:%d", SMTPHostname, SMTPPort), auth, envelopeSender, recipients, []byte(msg.String()))
}

// SendDebug returns a string describing the message that would be sent, and its recipients
func (msg *Message) SendDebug(envelopeSender string, recipients []string) string {
	out := fmt.Sprintf("------------------------------------------------------------\nSENDING MESSAGE FROM %s TO:\n", envelopeSender)
	for _, r := range recipients {
		out = out + fmt.Sprintf(" - %s\n", r)
	}
	out += fmt.Sprintf("MESSAGE:\n%s\n", msg.String())
	return out
}
