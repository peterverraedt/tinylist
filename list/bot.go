package list

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"net/mail"
	"strings"
	"time"
)

// A Config represents general configuration for a mailing list bot
type Config struct {
	CommandAddress string   `ini:"command_address"`
	BouncesAddress string   `ini:"bounces_address"`
	AdminAddresses []string `ini:"admin_addresses"`
	SMTPHostname   string   `ini:"smtp_hostname"`
	SMTPPort       uint64   `ini:"smtp_port"`
	SMTPUsername   string   `ini:"smtp_username"`
	SMTPPassword   string   `ini:"smtp_password"`
	Debug          bool     `ini:"debug"`
}

// A bot represents a mailing list bot
type bot struct {
	Config
	Lists      func() ([]*list, error)
	CreateList func(Definition) error
	ModifyList func(*list, Definition) error
	DeleteList func(*list) error
	LookupList func(string) (*list, error)
}

// Subscribe a given address to a listAddress
func (b *bot) Subscribe(address string, listAddress string, admin bool) (*list, error) {
	list, err := b.LookupList(listAddress)
	if err != nil {
		return nil, err
	}

	if list == nil {
		return nil, fmt.Errorf("Unable to subscribe to %s - it is not a valid mailing list", listAddress)
	}

	// Switch to id - in case we were passed address
	listAddress = list.Address

	subscription, err := list.IsSubscribed(address)
	if err != nil {
		return nil, err
	}
	if subscription != nil {
		return list, fmt.Errorf("You are already subscribed to %s", listAddress)
	}

	if list.Locked && !admin {
		return list, fmt.Errorf("List %s is locked, only admins can add subscribers", listAddress)
	}

	err = list.Subscribe(address)
	if err != nil {
		return list, fmt.Errorf("Subscription to %s failed with error: %s", listAddress, err.Error())
	}

	log.Printf("SUBSCRIPTION_CREATED User=%q List=%q\n", address, listAddress)
	return list, nil
}

// Unsubscribe a given address from a listAddress
func (b *bot) Unsubscribe(address string, listAddress string, admin bool) (*list, error) {
	list, err := b.LookupList(listAddress)
	if err != nil {
		return nil, err
	}

	if list == nil {
		return nil, fmt.Errorf("Unable to unsubscribe from %s - it is not a valid mailing list", listAddress)
	}

	// Switch to id - in case we were passed address
	listAddress = list.Address

	subscription, err := list.IsSubscribed(address)
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		return list, fmt.Errorf("You aren't subscribed to %s", listAddress)
	}

	// If a list is locked, set bounces instead to maximum
	if list.Locked && !admin {
		err = list.SetBounce(address, 65535, time.Now())
		if err != nil {
			return list, fmt.Errorf("Unsubscription to %s failed with error: %s", listAddress, err.Error())
		}
		log.Printf("UNSUBSCRIPTION_SET_BOUNCE User=%q List=%q\n", address, listAddress)
	} else {
		err = list.Unsubscribe(address)
		if err != nil {
			return list, fmt.Errorf("Unsubscription to %s failed with error: %s", listAddress, err.Error())
		}
		log.Printf("SUBSCRIPTION_REMOVED User=%q List=%q\n", address, listAddress)
	}
	return list, nil
}

// UnsubscribeAll unsubscribes a given address from all lists
func (b *bot) UnsubscribeAll(address string, admin bool) ([]*list, error) {
	lists, err := b.Lists()
	if err != nil {
		return nil, err
	}

	unsubscribed := []*list{}

	for _, list := range lists {
		subscription, err := list.IsSubscribed(address)
		if err != nil {
			return nil, err
		}
		if subscription == nil {
			continue
		}

		if list.Locked && !admin {
			err = list.SetBounce(address, 65535, time.Now())
			if err != nil {
				return nil, fmt.Errorf("Unsubscription to %s failed with error %s", list.Address, err.Error())
			}
			log.Printf("UNSUBSCRIPTION_SET_BOUNCE User=%q List=%q\n", address, list.Address)
		} else {
			err = list.Unsubscribe(address)
			if err != nil {
				return nil, fmt.Errorf("Unsubscription to %s failed with error %s", list.Address, err.Error())
			}
			log.Printf("SUBSCRIPTION_REMOVED User=%q List=%q\n", address, list.Address)
		}
		unsubscribed = append(unsubscribed, list)
	}

	if len(unsubscribed) == 0 {
		return nil, fmt.Errorf("Unable to unsubscribe %s from any list - no subcriptions found", address)
	}
	return unsubscribed, nil
}

// Handle a message from a io.Reader
// Only returns error if no error message could be sent to the user
func (b *bot) Handle(stream io.Reader) error {
	msg := &Message{}
	err := msg.FromReader(stream)
	if err != nil {
		return err
	}
	log.Printf("MESSAGE_RECEIVED Id=%q From=%q To=%q Cc=%q Bcc=%q Subject=%q\n",
		msg.Address, msg.From, msg.To, msg.Cc, msg.Bcc, msg.Subject)
	return b.HandleMessage(msg)
}

// HandleMessage handles a message
func (b *bot) HandleMessage(msg *Message) error {
	if b.isToCommandAddress(msg) {
		obj, err := mail.ParseAddress(msg.From)
		if err != nil {
			return err
		}
		obj.Address = strings.ToLower(obj.Address)

		reply, err := b.executeCommand(obj.Address, msg.Subject)
		if err != nil {
			log.Printf("COMMAND_FAILED From=%q Command=%q Error=%s\n", msg.From, msg.Subject, err.Error())

			if reply == "" {
				return b.reply(msg, fmt.Sprintf("Command failed: %s", err.Error()))
			}
		} else {
			log.Printf("COMMAND_SUCCEEDED From=%q Command=%q Message=%s\n", msg.From, msg.Subject, strings.Replace(reply, "\n", " ", -1))
		}

		return b.reply(msg, reply)
	}

	if br := b.isToBounceAddress(msg); br != nil {
		if br.Address == "" || br.List == "" {
			log.Printf("UNKNOWN_BOUNCE From=%q Subject=%s", msg.From, msg.Subject)
			return nil
		}
		err := b.handleBounce(br)
		if err != nil {
			log.Printf("BOUNCE_FAILED List=%q Address=%q Error=%s\n", br.List, br.Address, err.Error())
		} else {
			log.Printf("BOUNCE_HANDLED List=%q Address=%q\n", br.List, br.Address)
		}
		// Never return an error back to a bounce
		return nil
	}

	lists, err := b.lookupLists(msg)
	if err != nil {
		return err
	}
	if len(lists) > 0 {
		obj, err := mail.ParseAddress(msg.From)
		if err != nil {
			return err
		}
		obj.Address = strings.ToLower(obj.Address)

		// Go through all lists - don't stop at the first error!
		errors := map[string]error{}
		for _, list := range lists {
			if !list.CanPost(obj.Address) {
				log.Printf("UNAUTHORISED_POST From=%q To=%q Cc=%q Bcc=%q", msg.From, msg.To, msg.Cc, msg.Bcc)

				errors[list.Address] = fmt.Errorf("You are not an approved poster for this mailing list. Your message has not been delivered to %s", list.Address)

				continue
			}

			listMsg := msg.ResendAs(list, b.CommandAddress)

			if err := list.Archive(listMsg); err != nil {
				log.Printf("ARCHIVAL_FAILED listAddress=%q Id=%q From=%q To=%q Cc=%q Bcc=%q Subject=%q\n",
					list.Address, listMsg.Address, listMsg.From, listMsg.To, listMsg.Cc, listMsg.Bcc, listMsg.Subject)

				errors[list.Address] = err

				continue
			}

			if err := list.Send(listMsg, b.BouncesAddress, b.SMTPHostname, b.SMTPPort, b.SMTPUsername, b.SMTPPassword, b.Debug); err != nil {
				log.Printf("MESSAGE_FAILED listAddress=%q Id=%q From=%q To=%q Cc=%q Bcc=%q Subject=%q\n",
					list.Address, listMsg.Address, listMsg.From, listMsg.To, listMsg.Cc, listMsg.Bcc, listMsg.Subject)

				errors[list.Address] = err

				continue
			}

			log.Printf("MESSAGE_SENT listAddress=%q Id=%q From=%q To=%q Cc=%q Bcc=%q Subject=%q\n",
				list.Address, listMsg.Address, listMsg.From, listMsg.To, listMsg.Cc, listMsg.Bcc, listMsg.Subject)
		}

		// Check for errors
		if len(errors) > 0 {
			strs := []string{}
			for _, err := range errors {
				strs = append(strs, err.Error())
			}
			return b.reply(msg, strings.Join(strs, "\n"))
		}

		return nil
	}

	log.Printf("UNKNOWN_DESTINATION From=%q To=%q Cc=%q Bcc=%q", msg.From, msg.To, msg.Cc, msg.Bcc)
	return b.reply(msg, "No mailing lists addressed. Your message has not been delivered.")
}

// ExecuteCommand executes a command
func (b *bot) executeCommand(fromAddress string, subject string) (string, error) {
	admin := b.isAdmin(fromAddress)

	var buf bytes.Buffer
	cmd := NewCommand(admin, fromAddress, b, &buf)
	_, err := cmd.ParseString(subject)
	return buf.String(), err
}

func (b *bot) handleBounce(br *BounceResponse) error {
	list, err := b.LookupList(br.List)
	if err != nil {
		return err
	}
	if list == nil {
		return fmt.Errorf("Unknown list %s", br.List)
	}

	subscription, err := list.IsSubscribed(br.Address)
	if err != nil {
		return err
	}

	if subscription == nil {
		return fmt.Errorf("User %s is not subscribed to list %s", br.Address, br.List)
	}

	// Set or increase bounces
	bounces := subscription.Bounces

	if subscription.Bounces > 0 {
		// Remember bounces only for a limited interval
		period := time.Duration(math.Pow(2, float64(subscription.Bounces-1))) * BounceInterval
		effectiveCountUntil := subscription.LastBounce.Add(period)

		now := time.Now()
		if now.After(effectiveCountUntil) {
			bounces = 0
		}
	}

	// Increase bounces
	if bounces < 65535 {
		bounces++
	}

	return list.SetBounce(br.Address, bounces, time.Now())
}
