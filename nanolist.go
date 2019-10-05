package main

import (
	"net/mail"
	"fmt"
)



// Retrieve a list of mailing lists that are recipients of the given message
func lookupLists(msg *Message) []*List {
	lists := []*List{}

	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			list := lookupList(to.Address)
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	ccList, err := mail.ParseAddressList(msg.Cc)
	if err == nil {
		for _, cc := range ccList {
			list := lookupList(cc.Address)
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			list := lookupList(bcc.Address)
			if list != nil {
				lists = append(lists, list)
			}
		}
	}

	return lists
}

// Look up a mailing list by id or address
func lookupList(listKey string) *List {
	for _, list := range gConfig.Lists {
		if listKey == list.Id || listKey == list.Address {
			return list
		}
	}
	return nil
}

// Is the message bound for our command address?
func isToCommandAddress(msg *Message) bool {
	toList, err := mail.ParseAddressList(msg.To)
	if err == nil {
		for _, to := range toList {
			if to.Address == gConfig.CommandAddress {
				return true
			}
		}
	}

	ccList, err := mail.ParseAddressList(msg.Cc)
	if err == nil {
		for _, cc := range ccList {
			if cc.Address == gConfig.CommandAddress {
				return true
			}
		}
	}

	bccList, err := mail.ParseAddressList(msg.Bcc)
	if err == nil {
		for _, bcc := range bccList {
			if bcc.Address == gConfig.CommandAddress {
				return true
			}
		}
	}

	return false
}

// Generate an email-able list of commands
func commandInfo() string {
	return fmt.Sprintf("    help\r\n"+
		"      Information about valid commands\r\n"+
		"\r\n"+
		"    list\r\n"+
		"      Retrieve a list of available mailing lists\r\n"+
		"\r\n"+
		"    subscribe <list-id>\r\n"+
		"      Subscribe to <list-id>\r\n"+
		"\r\n"+
		"    unsubscribe <list-id>\r\n"+
		"      Unsubscribe from <list-id>\r\n"+
		"\r\n"+
		"To send a command, email %s with the command as the subject.\r\n",
		gConfig.CommandAddress)
}

