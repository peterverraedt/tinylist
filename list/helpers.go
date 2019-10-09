package list

import (
	"bytes"
	"fmt"
	"net/mail"
	"strings"
)

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

// A BounceResponse represents an incoming mail on the bounce address with a list and an address specified as parameter
type BounceResponse struct {
	BounceAddress string
	List          string
	Address       string
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
		br.Address = fmt.Sprintf("%s@%s", parts[2][:i], parts[2][i+1:])
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

func (b *Bot) commandInfo() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Available commands:\n\n")
	fmt.Fprintf(&buf, "    help\n")
	fmt.Fprintf(&buf, "      Information about valid commands\n\n")
	fmt.Fprintf(&buf, "    list\n")
	fmt.Fprintf(&buf, "      Retrieve a list of available mailing lists\n\n")
	fmt.Fprintf(&buf, "    subscribe <list-address>\n")
	fmt.Fprintf(&buf, "      Subscribe to <list-address>\n\n")
	fmt.Fprintf(&buf, "    unsubscribe <list-address>\n")
	fmt.Fprintf(&buf, "      Unsubscribe to <list-address>\n\n")
	fmt.Fprintf(&buf, "    unsubscribe\n")
	fmt.Fprintf(&buf, "      Unsubscribe to all lists\n\n")
	fmt.Fprintf(&buf, "To send a command, email %s with the command as the subject.", b.CommandAddress)

	return buf.String()
}

func (b *Bot) reply(msg *Message, message string) error {
	message = strings.Replace(message, "\n", "\r\n", -1)
	message = fmt.Sprintf("%s\r\n", message)

	reply := msg.Reply()
	reply.From = b.CommandAddress
	reply.Body = []byte(message)

	return reply.Send(b.CommandAddress, []string{msg.From}, b.SMTPHostname, b.SMTPPort, b.SMTPUsername, b.SMTPPassword, b.Debug)
}
