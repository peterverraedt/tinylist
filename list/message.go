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
	"sort"
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
	Sender          string
	Address         string
	InReplyTo       string
	Precedence      string
	ListID          string
	ListUnsubscribe string
	ListSubscribe   string
	ListArchive     string
	ListOwner       string
	ListHelp        string
	XMailingList    string
	XLoop           string
	MIMEVersion     string
	ContentType     string
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
	msg.To = header.Get("To")
	msg.Cc = header.Get("Cc")
	msg.Bcc = header.Get("Bcc")
	msg.Date = header.Get("Date")
	msg.Sender = header.Get("Sender")
	msg.Address = header.Get("Message-Id")
	msg.InReplyTo = header.Get("In-Reply-To")
	msg.Precedence = header.Get("Precedence")
	msg.ListID = header.Get("List-Id")
	msg.ListUnsubscribe = header.Get("List-Unsubscribe")
	msg.ListSubscribe = header.Get("List-Subscribe")
	msg.ListOwner = header.Get("List-Owner")
	msg.ListArchive = header.Get("List-Archive")
	msg.ListHelp = header.Get("List-Help")
	msg.XMailingList = header.Get("X-Mailing-List")
	msg.XLoop = header.Get("X-Loop")
	msg.MIMEVersion = header.Get("MIME-Version")
	msg.ContentType = header.Get("Content-Type")
	msg.Body = body

	header.Del("Subject")
	header.Del("From")
	header.Del("To")
	header.Del("Cc")
	header.Del("Bcc")
	header.Del("Date")
	header.Del("Sender")
	header.Del("Message-Id")
	header.Del("In-Reply-To")
	header.Del("Precedence")
	header.Del("List-Id")
	header.Del("List-Unsubscribe")
	header.Del("List-Subscribe")
	header.Del("List-Owner")
	header.Del("List-Archive")
	header.Del("List-Help")
	header.Del("X-Mailing-List")
	header.Del("X-Loop")
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
	reply.InReplyTo = msg.Address
	reply.Date = time.Now().Format("Mon, 2 Jan 2006 15:04:05 -0700")
	reply.MIMEVersion = "1.0"
	reply.ContentType = "text/plain; charset=utf-8"
	reply.Headers = map[string][]string{}
	reply.Body = []byte{}
	return reply
}

// ResendAs a list prepares a copy of the message to be used for a list forward
func (msg *Message) ResendAs(list *list, commandAddress string) *Message {
	send := &Message{}

	listID := fmt.Sprintf("%s <%s>", list.Name, list.Address)

	send.Subject = msg.Subject
	send.From = msg.From
	send.To = msg.To
	send.Cc = msg.Cc
	send.Date = msg.Date
	send.Sender = listID
	send.Address = msg.Address
	send.InReplyTo = msg.InReplyTo
	send.Precedence = "bulk"
	send.ListID = listID
	send.ListUnsubscribe = fmt.Sprintf("<mailto:%s?subject=unsubscribe%%20%s>", commandAddress, list.Address)
	send.ListSubscribe = fmt.Sprintf("<mailto:%s?subject=subscribe%%20%s>", commandAddress, list.Address)
	send.XMailingList = listID
	send.XLoop = listID
	send.MIMEVersion = msg.MIMEVersion
	send.ContentType = msg.ContentType
	send.Body = msg.Body

	// If the destination mailing list is in the Bcc field, keep it there
	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			bcc.Address = strings.ToLower(bcc.Address)
			if bcc.Address == list.Address {
				send.Bcc = list.Name + " <" + list.Address + ">"
				break
			}
		}
	}

	// Copy other headers unmodified (e.g. DKIM signatures)
	send.Headers = map[string][]string{}
	for key, values := range msg.Headers {
		canonicKey := textproto.CanonicalMIMEHeaderKey(key)

		// Filter keys
		switch canonicKey {
		case "Received":
			continue
		case "X-Original-To":
			continue
		case "X-Received":
			continue
		case "Delivered-To":
			continue
		case "Return-Path":
			continue
		case "Arc-Authentication-Results":
			continue
		case "Arc-Message-Signature":
			continue
		case "Arc-Seal":
			continue
		case "X-Spamd-Result":
			continue
		case "X-Rspamd-Server":
			continue
		}

		// Keys with spaces are probably malformed
		if strings.Index(canonicKey, " ") >= 0 {
			continue
		}

		for index, value := range values {
			// If value contains newline, strip it
			i := strings.IndexAny(value, "\r\n")
			if i >= 0 {
				values[index] = value[:i]
			}
		}

		send.Headers[canonicKey] = values
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
	if len(msg.Sender) > 0 {
		fmt.Fprintf(&buf, "Sender: %s\r\n", msg.Sender)
	}
	if len(msg.Address) > 0 {
		fmt.Fprintf(&buf, "Message-Id: %s\r\n", msg.Address)
	}
	if len(msg.InReplyTo) > 0 {
		fmt.Fprintf(&buf, "In-Reply-To: %s\r\n", msg.InReplyTo)
	}
	if len(msg.Precedence) > 0 {
		fmt.Fprintf(&buf, "Precedence: %s\r\n", msg.Precedence)
	}
	if len(msg.ListID) > 0 {
		fmt.Fprintf(&buf, "List-Id: %s\r\n", msg.ListID)
	}
	if len(msg.ListUnsubscribe) > 0 {
		fmt.Fprintf(&buf, "List-Unsubscribe: %s\r\n", msg.ListUnsubscribe)
	}
	if len(msg.ListSubscribe) > 0 {
		fmt.Fprintf(&buf, "List-Subscribe: %s\r\n", msg.ListSubscribe)
	}
	if len(msg.ListOwner) > 0 {
		fmt.Fprintf(&buf, "List-Owner: %s\r\n", msg.ListOwner)
	}
	if len(msg.ListArchive) > 0 {
		fmt.Fprintf(&buf, "List-Archive: %s\r\n", msg.ListArchive)
	}
	if len(msg.ListHelp) > 0 {
		fmt.Fprintf(&buf, "List-Help: %s\r\n", msg.ListHelp)
	}
	if len(msg.XMailingList) > 0 {
		fmt.Fprintf(&buf, "X-Mailing-List: %s\r\n", msg.XMailingList)
	}
	if len(msg.XLoop) > 0 {
		fmt.Fprintf(&buf, "X-Loop: %s\r\n", msg.XLoop)
	}

	extraKeys := []string{}
	for key := range msg.Headers {
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)
	for _, key := range extraKeys {
		for _, value := range msg.Headers[key] {
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
func (msg *Message) SendVERP(envelopeSender string, recipients []string, SMTPHostname string, SMTPPort uint64, SMTPUsername string, SMTPPassword string, debug bool) error {
	parts := strings.SplitN(envelopeSender, "@", 2)
	if len(parts) < 2 {
		return fmt.Errorf("Invalid envelope sender %s", envelopeSender)
	}

	errors := []error{}
	for _, recipient := range recipients {
		envelope := fmt.Sprintf("%s+%s@%s", parts[0], strings.Replace(recipient, "@", "=", 1), parts[1])
		err := msg.Send(envelope, []string{recipient}, SMTPHostname, SMTPPort, SMTPUsername, SMTPPassword, debug)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d errors occurred: %v", len(errors), errors)
	}
	return nil
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
