package list

import (
	"fmt"
	"net/mail"
	"strings"
)

func (b *bot) isToCommandAddress(msg *Message) bool {
	if msg.XOriginalTo != "" {
		return strings.ToLower(msg.XOriginalTo) == b.CommandAddress
	}

	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			to.Address = strings.ToLower(to.Address)
			if to.Address == b.CommandAddress {
				return true
			}
		}
	}

	ccList, err := mail.ParseAddressList(msg.Cc)
	if err == nil {
		for _, cc := range ccList {
			cc.Address = strings.ToLower(cc.Address)
			if cc.Address == b.CommandAddress {
				return true
			}
		}
	}

	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			bcc.Address = strings.ToLower(bcc.Address)
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

func (b *bot) isToBounceAddress(msg *Message) *BounceResponse {
	if msg.XOriginalTo != "" {
		br := parseBounce(strings.ToLower(msg.XOriginalTo))
		if br != nil && br.BounceAddress == b.BouncesAddress {
			return br
		}

		return nil
	}

	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			to.Address = strings.ToLower(to.Address)
			br := parseBounce(to.Address)
			if br != nil && br.BounceAddress == b.BouncesAddress {
				return br
			}
		}
	}

	ccList, err := mail.ParseAddressList(msg.Cc)
	if err == nil {
		for _, cc := range ccList {
			cc.Address = strings.ToLower(cc.Address)
			br := parseBounce(cc.Address)
			if br != nil && br.BounceAddress == b.BouncesAddress {
				return br
			}
		}
	}

	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			bcc.Address = strings.ToLower(bcc.Address)
			br := parseBounce(bcc.Address)
			if br != nil && br.BounceAddress == b.BouncesAddress {
				return br
			}
		}
	}

	return nil
}

// Retrieve a list of mailing lists that are recipients of the given message
func (b *bot) lookupLists(msg *Message) ([]*list, error) {
	lists := []*list{}

	if msg.XOriginalTo != "" {
		list, err := b.LookupList(strings.ToLower(msg.XOriginalTo))
		if err != nil {
			return nil, err
		}
		if list != nil {
			lists = append(lists, list)
		}

		return lists, nil
	}

	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			to.Address = strings.ToLower(to.Address)
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
			cc.Address = strings.ToLower(cc.Address)
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
			bcc.Address = strings.ToLower(bcc.Address)
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

func (b *bot) isAdmin(address string) bool {
	for _, a := range b.AdminAddresses {
		if a == address {
			return true
		}
	}
	return false
}

func (b *bot) reply(msg *Message, message string) error {
	message = strings.Replace(message, "\n", "\r\n", -1)
	message = fmt.Sprintf("%s\r\n", message)

	reply := msg.Reply()
	reply.From = b.CommandAddress
	reply.Body = []byte(message)

	return reply.Send(b.CommandAddress, []string{msg.From}, b.SMTPHostname, b.SMTPPort, b.SMTPUsername, b.SMTPPassword, b.Debug)
}
