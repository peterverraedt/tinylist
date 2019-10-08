package list

import (
	"fmt"
	"io"
	"log"
	"net/mail"
	"strings"
	"time"
)

// A Bot represents a mailing list bot
type Bot struct {
	CommandAddress string `ini:"command_address"`
	BouncesAddress string `ini:"bounces_address"`
	SMTPHostname   string `ini:"smtp_hostname"`
	SMTPPort       uint64 `ini:"smtp_port"`
	SMTPUsername   string `ini:"smtp_username"`
	SMTPPassword   string `ini:"smtp_password"`
	Lists          func() ([]*List, error)
	LookupList     func(string) (*List, error)
	Debug          bool
}

// Subscribe a given address to a listID
func (b *Bot) Subscribe(address string, listID string, admin bool) (*List, error) {
	list, err := b.LookupList(listID)
	if err != nil {
		return nil, err
	}

	if list == nil {
		log.Printf("INVALID_SUBSCRIPTION_REQUEST User=%q List=%q\n", address, listID)
		return nil, fmt.Errorf("Unable to subscribe to %s - it is not a valid mailing list", listID)
	}

	// Switch to id - in case we were passed address
	listID = list.ID

	subscription, err := list.IsSubscribed(address)
	if err != nil {
		return nil, err
	}
	if subscription != nil {
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

	log.Printf("SUBSCRIPTION_ADDED User=%q List=%q\n", address, listID)
	return list, nil
}

// Unsubscribe a given address from a listID
func (b *Bot) Unsubscribe(address string, listID string, admin bool) (*List, error) {
	list, err := b.LookupList(listID)
	if err != nil {
		return nil, err
	}

	if list == nil {
		log.Printf("INVALID_UNSUBSCRIPTION_REQUEST User=%q List=%q\n", address, listID)
		return nil, fmt.Errorf("Unable to unsubscribe from %s - it is not a valid mailing list", listID)
	}

	// Switch to id - in case we were passed address
	listID = list.ID

	subscription, err := list.IsSubscribed(address)
	if err != nil {
		return nil, err
	}
	if subscription == nil {
		log.Printf("DUPLICATE_UNSUBSCRIPTION_REQUEST User=%q List=%q\n", address, listID)
		return list, fmt.Errorf("You aren't subscribed to %s", listID)
	}

	// If a list is locked, set bounces instead to maximum
	if list.Locked && !admin {
		err = list.SetBounce(address, 65535, time.Now())
		if err != nil {
			log.Printf("DIVERTED_UNSUBSCRIPTION_FAILED User=%q List=%q Error=%s\n", address, listID, err.Error())
			return list, fmt.Errorf("Unsubscription to %s failed with error %s", listID, err.Error())
		}
		log.Printf("UNSUBSCRIPTION_REQUEST_DIVERTED User=%q List=%q\n", address, listID)
	} else {
		err = list.Unsubscribe(address)
		if err != nil {
			log.Printf("UNSUBSCRIPTION_FAILED User=%q List=%q Error=%s\n", address, listID, err.Error())
			return list, fmt.Errorf("Unsubscription to %s failed with error %s", listID, err.Error())
		}
		log.Printf("SUBSCRIPTION_REMOVED User=%q List=%q\n", address, listID)
	}
	return list, nil
}

// Unsubscribe a given address from a listID
func (b *Bot) UnsubscribeAll(address string, admin bool) ([]*List, error) {
	lists, err := b.Lists()
	if err != nil {
		return nil, err
	}

	unsubscribed := []*List{}

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
				log.Printf("DIVERTED_UNSUBSCRIPTION_FAILED User=%q List=%q Error=%s\n", address, list.ID, err.Error())
				return nil, fmt.Errorf("Unsubscription to %s failed with error %s", list.ID, err.Error())
			}
			log.Printf("UNSUBSCRIPTION_REQUEST_DIVERTED User=%q List=%q\n", address, list.ID)
		} else {
			err = list.Unsubscribe(address)
			if err != nil {
				log.Printf("UNSUBSCRIPTION_FAILED User=%q List=%q Error=%s\n", address, list.ID, err.Error())
				return nil, fmt.Errorf("Unsubscription to %s failed with error %s", list.ID, err.Error())
			}
			log.Printf("SUBSCRIPTION_REMOVED User=%q List=%q\n", address, list.ID)
		}
		unsubscribed = append(unsubscribed, list)
	}

	if len(unsubscribed) == 0 {
		log.Printf("INVALID_UNSUBSCRIPTION_REQUEST User=%q List=ALL\n", address)
		return nil, fmt.Errorf("Unable to unsubscribe %s from any list - no subcriptions found", address)
	}
	return unsubscribed, nil
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
	if br := b.isToBounceAddress(msg); br != nil {
		return b.handleBounce(msg, br)
	}
	lists, err := b.lookupLists(msg)
	if err != nil {
		return err
	}
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

			lists, err := b.Lists()
			if err != nil {
				return err
			}

			lines := []string{"Available mailing lists:", ""}
			for _, list := range lists {
				if !list.Hidden {
					lines = append(lines,
						fmt.Sprintf("ID: %s", list.ID),
						fmt.Sprintf("Name: %s", list.Name),
						fmt.Sprintf("Description: %s", list.Description),
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
				obj, err := mail.ParseAddress(msg.From)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				list, err := b.Subscribe(obj.Address, parts[1], false)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				return b.reply(msg, fmt.Sprintf("You are now subscribed to %s", list.ID))
			}
		case "unsubscribe":
			if len(parts) > 2 {
				obj, err := mail.ParseAddress(msg.From)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				list, err := b.Unsubscribe(obj.Address, parts[1], false)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				return b.reply(msg, fmt.Sprintf("You are now unsubscribed from %s", list.ID))
			} else if len(parts) == 1 {
				obj, err := mail.ParseAddress(msg.From)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				lists, err := b.UnsubscribeAll(obj.Address, false)
				if err != nil {
					return b.reply(msg, err.Error())
				}
				listIDs := []string{}
				for _, list := range lists {
					listIDs = append(listIDs, list.ID)
				}
				return b.reply(msg, fmt.Sprintf("You are now unsubscribed from %s", strings.Join(listIDs, ", ")))
			}
		}
	}
	log.Printf("UNKNOWN_COMMAND From=%q Command=%s", msg.From, msg.Subject)
	return b.replyLines(msg, append([]string{
		fmt.Sprintf("%s is not a valid command.", msg.Subject),
		"Valid commands are:",
	}, b.commandInfo()...))
}

func (b *Bot) handleBounce(msg *Message, br *BounceResponse) error {
	if br.Address == "" || br.List == "" {
		log.Printf("UNKNOWN_BOUNCE From=%q Subject=%s", msg.From, msg.Subject)
		return nil
	}

	list, err := b.LookupList(br.List)
	if err != nil {
		log.Printf("BOUNCE_LOOKUP_ERROR User=%q List=%s Error=%s", br.Address, br.List, err.Error())
		return err
	}
	if list == nil {
		log.Printf("BOUNCE_UNKNOWN_LIST User=%q List=%q\n", br.Address, br.List)
		return nil
	}

	subscription, err := list.IsSubscribed(br.Address)
	if err != nil {
		log.Printf("BOUNCE_LOOKUP_ERROR User=%q List=%q Error=%s", br.Address, br.List, err.Error())
		return err
	}

	if subscription == nil {
		log.Printf("BOUNCE_UNSUBSCRIBED_USER User=%q List=%q\n", br.Address, br.List)
		return nil
	}

	// Increase bounces
	bounces := subscription.Bounces
	if bounces < 65535 {
		bounces++
	}
	
	err = list.SetBounce(br.Address, bounces, time.Now())
	if err != nil {
		log.Printf("BOUNCE_SET_ERROR User=%q List=%q Error=%s\n", br.Address, br.List, err.Error())
	}
	return err
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

type BounceResponse struct {
	BounceAddress string
	List string
	Address string
}

func parseBounce(address string) *BounceResponse {
	splitDomain := strings.SplitN(address, "@", 2)
	if len(splitDomain) < 2 {
		return nil
	}

	br := &BounceResponse{}

	parts := strings.SplitN(splitDomain[0], "+", 3)
	br.BounceAddress = fmt.Sprintf("%s@%s", parts[0], splitDomain[1])

	if len(parts) > 1 {
		i := strings.LastIndex(parts[1], "=")
		br.List = fmt.Sprintf("%s@%s", parts[1][:i], parts[1][i+1:])
	}

	if len(parts) > 2 {
		i := strings.LastIndex(parts[2], "=")
		br.Address = fmt.Sprintf("%s@%s", parts[1][:i], parts[1][i+1:])
	}

	return br
}

func (b *Bot) isToBounceAddress(msg *Message) *BounceResponse {
	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			br := parseBounce(to.Address)
			if br != nil && br.BounceAddress == b.BouncesAddress {
				return br
			}
		}
	}

	ccList, err := mail.ParseAddressList(msg.Cc)
	if err == nil {
		for _, cc := range ccList {
			br := parseBounce(cc.Address)
			if br != nil && br.BounceAddress == b.BouncesAddress {
				return br
			}
		}
	}

	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			br := parseBounce(bcc.Address)
			if br != nil && br.BounceAddress == b.BouncesAddress {
				return br
			}
		}
	}

	return nil
}

// Retrieve a list of mailing lists that are recipients of the given message
func (b *Bot) lookupLists(msg *Message) ([]*List, error) {
	lists := []*List{}

	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			list, err := b.LookupList(to.Address)
			if err != nil {
				return nil, err
			}
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	ccList, err := mail.ParseAddressList(msg.Cc)
	if err == nil {
		for _, cc := range ccList {
			list, err := b.LookupList(cc.Address)
			if err != nil {
				return nil, err
			}
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			list, err := b.LookupList(bcc.Address)
			if err != nil {
				return nil, err
			}
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	return lists, nil
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
