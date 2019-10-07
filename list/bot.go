package list

import (
	"fmt"
	"io"
	"log"
	"net/mail"
	"strings"
)

// A Bot represents a mailing list bot
type Bot struct {
	CommandAddress string `ini:"command_address"`
	BouncesAddress string `ini:"bounces_address"`
	SMTPHostname   string `ini:"smtp_hostname"`
	SMTPPort       uint64 `ini:"smtp_port"`
	SMTPUsername   string `ini:"smtp_username"`
	SMTPPassword   string `ini:"smtp_password"`
	Lists          map[string]*List
	Debug          bool
}

// Subscribe a given address to a listID
func (b *Bot) Subscribe(address string, listID string, admin bool) (*List, error) {
	list := b.lookupList(listID)

	if list == nil {
		log.Printf("INVALID_SUBSCRIPTION_REQUEST User=%q List=%q\n", address, listID)
		return nil, fmt.Errorf("Unable to subscribe to %s - it is not a valid mailing list", listID)
	}

	// Switch to id - in case we were passed address
	listID = list.ID

	ok, err := list.IsSubscribed(address)
	if err != nil {
		return nil, err
	}
	if ok {
		log.Printf("DUPLICATE_SUBSCRIPTION_REQUEST User=%q List=%q\n", address, listID)
		return list, fmt.Errorf("You are already subscribed to %s", listID)
	}

	if list.Locked && !admin {
		log.Printf("SUBSCRIPTION_REQUEST_BLOCKED User=%q List=%q\n", address, listID)
		return list, fmt.Errorf("List %s is locked, only admins can add subscribers", listID)
	}

	err = list.Subscribe(address)
	if err != nil {
		log.Printf("SUBSCRIPTION_FAILED User=%q List=%q Error=%s\n", address, listID, err.Error())
		return list, fmt.Errorf("Subscription to %s failed with error %s", listID, err.Error())
	}
	return list, nil
}

// Unsubscribe a given address from a listID
func (b *Bot) Unsubscribe(address string, listID string, admin bool) (*List, error) {
	list := b.lookupList(listID)

	if list == nil {
		log.Printf("INVALID_UNSUBSCRIPTION_REQUEST User=%q List=%q\n", address, listID)
		return nil, fmt.Errorf("Unable to unsubscribe from %s - it is not a valid mailing list", listID)
	}

	// Switch to id - in case we were passed address
	listID = list.ID

	ok, err := list.IsSubscribed(address)
	if err != nil {
		return nil, err
	}
	if !ok {
		log.Printf("DUPLICATE_UNSUBSCRIPTION_REQUEST User=%q List=%q\n", address, listID)
		return list, fmt.Errorf("You aren't subscribed to %s", listID)
	}

	if list.Locked && !admin {
		log.Printf("UNSUBSCRIPTION_REQUEST_BLOCKED User=%q List=%q\n", address, listID)
		return list, fmt.Errorf("List %s is locked, only admins can remove subscribers", listID)
	}

	err = list.Unsubscribe(address)
	if err != nil {
		log.Printf("UNSUBSCRIPTION_FAILED User=%q List=%q Error=%s\n", address, listID, err.Error())
		return list, fmt.Errorf("Unsubscription to %s failed with error %s", listID, err.Error())
	}
	return list, nil
}

// Handle a message from a io.Reader
func (b *Bot) Handle(stream io.Reader) error {
	msg := &Message{}
	err := msg.FromReader(stream)
	if err != nil {
		return err
	}
	log.Printf("MESSAGE_RECEIVED Id=%q From=%q To=%q Cc=%q Bcc=%q Subject=%q\n",
		msg.ID, msg.From, msg.To, msg.Cc, msg.Bcc, msg.Subject)
	return b.HandleMessage(msg)
}

// HandleMessage handles a message
func (b *Bot) HandleMessage(msg *Message) error {
	if b.isToCommandAddress(msg) {
		return b.handleCommand(msg)
	}
	lists := b.lookupLists(msg)
	if len(lists) > 0 {
		for _, list := range lists {
			if list.CanPost(msg.From) {
				listMsg := msg.ResendAs(list, b.CommandAddress)
				err := list.Send(listMsg, b.BouncesAddress, b.SMTPHostname, b.SMTPPort, b.SMTPUsername, b.SMTPPassword, b.Debug)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				log.Printf("MESSAGE_SENT listID=%q Id=%q From=%q To=%q Cc=%q Bcc=%q Subject=%q\n",
					list.ID, listMsg.ID, listMsg.From, listMsg.To, listMsg.Cc, listMsg.Bcc, listMsg.Subject)
			} else {
				log.Printf("UNAUTHORISED_POST From=%q To=%q Cc=%q Bcc=%q", msg.From, msg.To, msg.Cc, msg.Bcc)
				return b.reply(msg, fmt.Sprintf("You are not an approved poster for this mailing list. Your message has not been delivered to %s.", list.ID))
			}
		}
		return nil
	}
	log.Printf("UNKNOWN_DESTINATION From=%q To=%q Cc=%q Bcc=%q", msg.From, msg.To, msg.Cc, msg.Bcc)
	return b.reply(msg, "No mailing lists addressed. Your message has not been delivered.")
}

func (b *Bot) handleCommand(msg *Message) error {
	parts := strings.Split(msg.Subject, " ")
	if len(parts) > 1 {
		switch parts[0] {
		case "lists":
			log.Printf("LISTS_ISSUED From=%q", msg.From)

			lines := []string{"Available mailing lists:", ""}
			for _, list := range b.Lists {
				if !list.Hidden {
					lines = append(lines,
						fmt.Sprintf("Id: %s", list.ID),
						fmt.Sprintf("Name: %s", list.Name),
						fmt.Sprintf("Description: %s", list.Description),
						fmt.Sprintf("Address: %s", list.Address),
						"")
				}
			}
			lines = append(lines, "", fmt.Sprintf("To subscribe to a mailing list, email %s with 'subscribe <list-id>' as the subject.", b.CommandAddress))

			return b.replyLines(msg, lines)
		case "help":
			log.Printf("HELP_ISSUED From=%q", msg.From)
			return b.replyLines(msg, b.commandInfo())
		case "subscribe":
			if len(parts) > 2 {
				list, err := b.Subscribe(msg.From, parts[1], false)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				return b.reply(msg, fmt.Sprintf("You are now subscribed to %s", list.ID))
			}
		case "unsubscribe":
			if len(parts) > 2 {
				list, err := b.Unsubscribe(msg.From, parts[1], false)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				return b.reply(msg, fmt.Sprintf("You are now unsubscribed from %s", list.ID))
			}
		}
	}
	log.Printf("UNKNOWN_COMMAND From=%q Command=%s", msg.From, msg.Subject)
	return b.replyLines(msg, append([]string{
		fmt.Sprintf("%s is not a valid command.", msg.Subject),
		"Valid commands are:",
	}, b.commandInfo()...))
}

func (b *Bot) commandInfo() []string {
	return []string{
		"    help",
		"      Information about valid commands",
		"",
		"    list",
		"      Retrieve a list of available mailing lists",
		"",
		"    subscribe <list-id>",
		"      Subscribe to <list-id>",
		"",
		"    unsubscribe <list-id>",
		"      Unsubscribe from <list-id>",
		"",
		fmt.Sprintf("To send a command, email %s with the command as the subject.", b.CommandAddress),
	}
}

func (b *Bot) isToCommandAddress(msg *Message) bool {
	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			if to.Address == b.CommandAddress {
				return true
			}
		}
	}

	ccList, err := mail.ParseAddressList(msg.Cc)
	if err == nil {
		for _, cc := range ccList {
			if cc.Address == b.CommandAddress {
				return true
			}
		}
	}

	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			if bcc.Address == b.CommandAddress {
				return true
			}
		}
	}

	return false
}

// Retrieve a list of mailing lists that are recipients of the given message
func (b *Bot) lookupLists(msg *Message) []*List {
	lists := []*List{}

	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			list := b.lookupList(to.Address)
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	ccList, err := mail.ParseAddressList(msg.Cc)
	if err == nil {
		for _, cc := range ccList {
			list := b.lookupList(cc.Address)
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			list := b.lookupList(bcc.Address)
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	return lists
}

// Look up a mailing list by id or address
func (b *Bot) lookupList(listKey string) *List {
	for _, list := range b.Lists {
		if listKey == list.ID || listKey == list.Address {
			return list
		}
	}
	return nil
}

func (b *Bot) reply(msg *Message, message string) error {
	return b.replyLines(msg, []string{message})
}

func (b *Bot) replyLines(msg *Message, body []string) error {
	reply := msg.Reply()
	reply.From = b.CommandAddress
	reply.Body = fmt.Sprintf("%s\r\n", strings.Join(body, "\r\n"))
	return reply.Send(b.CommandAddress, []string{msg.From}, b.SMTPHostname, b.SMTPPort, b.SMTPUsername, b.SMTPPassword, b.Debug)
}
