package main

import (
	"strings"
	"log"
	"fmt"
	"bytes"
	"os"
)

// Handle the command given by the user
func handleCommand(msg *Message) {
	if msg.Subject == "lists" {
		handleShowLists(msg)
	} else if msg.Subject == "help" {
		handleHelp(msg)
	} else if strings.HasPrefix(msg.Subject, "subscribe") {
		handleSubscribe(msg)
	} else if strings.HasPrefix(msg.Subject, "unsubscribe") {
		handleUnsubscribe(msg)
	} else {
		handleUnknownCommand(msg)
	}
}

// Reply to a message that has nowhere to go
func handleNoDestination(msg *Message) {
	reply := msg.Reply()
	reply.From = gConfig.CommandAddress
	reply.Body = "No mailing lists addressed. Your message has not been delivered.\r\n"
	reply.Send([]string{msg.From})
	log.Printf("UNKNOWN_DESTINATION From=%q To=%q Cc=%q Bcc=%q", msg.From, msg.To, msg.Cc, msg.Bcc)
}

// Reply that the user isn't authorised to post to the list
func handleNotAuthorisedToPost(msg *Message) {
	reply := msg.Reply()
	reply.From = gConfig.CommandAddress
	reply.Body = "You are not an approved poster for this mailing list. Your message has not been delivered.\r\n"
	reply.Send([]string{msg.From})
	log.Printf("UNAUTHORISED_POST From=%q To=%q Cc=%q Bcc=%q", msg.From, msg.To, msg.Cc, msg.Bcc)
}

// Reply to an unknown command, giving some help
func handleUnknownCommand(msg *Message) {
	reply := msg.Reply()
	reply.From = gConfig.CommandAddress
	reply.Body = fmt.Sprintf(
		"%s is not a valid command.\r\n\r\n"+
			"Valid commands are:\r\n\r\n"+
			commandInfo(),
		msg.Subject)
	reply.Send([]string{msg.From})
	log.Printf("UNKNOWN_COMMAND From=%q", msg.From)
}

// Reply to a help command with help information
func handleHelp(msg *Message) {
	var body bytes.Buffer
	fmt.Fprintf(&body, commandInfo())
	reply := msg.Reply()
	reply.From = gConfig.CommandAddress
	reply.Body = body.String()
	reply.Send([]string{msg.From})
	log.Printf("HELP_SENT To=%q", reply.To)
}

// Reply to a show mailing lists command with a list of mailing lists
func handleShowLists(msg *Message) {
	var body bytes.Buffer
	fmt.Fprintf(&body, "Available mailing lists:\r\n\r\n")
	for _, list := range gConfig.Lists {
		if !list.Hidden {
			fmt.Fprintf(&body,
				"Id: %s\r\n"+
					"Name: %s\r\n"+
					"Description: %s\r\n"+
					"Address: %s\r\n\r\n",
				list.Id, list.Name, list.Description, list.Address)
		}
	}

	fmt.Fprintf(&body,
		"\r\nTo subscribe to a mailing list, email %s with 'subscribe <list-id>' as the subject.\r\n",
		gConfig.CommandAddress)

	reply := msg.Reply()
	reply.From = gConfig.CommandAddress
	reply.Body = body.String()
	reply.Send([]string{msg.From})
	log.Printf("LIST_SENT To=%q", reply.To)
}

// Handle a subscribe command
func handleSubscribe(msg *Message) {
	listId := strings.TrimPrefix(msg.Subject, "subscribe ")
	list := lookupList(listId)

	if list == nil {
		reply := msg.Reply()
		reply.Body = fmt.Sprintf("Unable to subscribe to %s  - it is not a valid mailing list.\r\n", listId)
		reply.Send([]string{msg.From})
		log.Printf("INVALID_SUBSCRIPTION_REQUEST User=%q List=%q\n", msg.From, listId)
		os.Exit(0)
	}

	// Switch to id - in case we were passed address
	listId = list.Id

	if isSubscribed(msg.From, listId) {
		reply := msg.Reply()
		reply.Body = fmt.Sprintf("You are already subscribed to %s\r\n", listId)
		reply.Send([]string{msg.From})
		log.Printf("DUPLICATE_SUBSCRIPTION_REQUEST User=%q List=%q\n", msg.From, listId)
		os.Exit(0)
	}

	if list.Locked {
		reply := msg.Reply()
		reply.Body = fmt.Sprintf("List %s is locked, only admins can add subscribers\r\n", listId)
		reply.Send([]string{msg.From})
		log.Printf("SUBSCRIPTION_REQUEST_BLOCKED User=%q List=%q\n", msg.From, listId)
		os.Exit(0)
	}

	addSubscription(msg.From, listId)
	reply := msg.Reply()
	reply.Body = fmt.Sprintf("You are now subscribed to %s\r\n", listId)
	reply.Send([]string{msg.From})
}

// Handle an unsubscribe command
func handleUnsubscribe(msg *Message) {
	listId := strings.TrimPrefix(msg.Subject, "unsubscribe ")
	list := lookupList(listId)

	if list == nil {
		reply := msg.Reply()
		reply.Body = fmt.Sprintf("Unable to unsubscribe from %s  - it is not a valid mailing list.\r\n", listId)
		reply.Send([]string{msg.From})
		log.Printf("INVALID_UNSUBSCRIPTION_REQUEST User=%q List=%q\n", msg.From, listId)
		os.Exit(0)
	}

	// Switch to id - in case we were passed address
	listId = list.Id

	if !isSubscribed(msg.From, listId) {
		reply := msg.Reply()
		reply.Body = fmt.Sprintf("You aren't subscribed to %s\r\n", listId)
		reply.Send([]string{msg.From})
		log.Printf("DUPLICATE_UNSUBSCRIPTION_REQUEST User=%q List=%q\n", msg.From, listId)
		os.Exit(0)
	}

	if list.Locked {
		reply := msg.Reply()
		reply.Body = fmt.Sprintf("List %s is locked, only admins can remove subscribers\r\n", listId)
		reply.Send([]string{msg.From})
		log.Printf("UNSUBSCRIPTION_REQUEST_BLOCKED User=%q List=%q\n", msg.From, listId)
		os.Exit(0)
	}

	removeSubscription(msg.From, listId)
	reply := msg.Reply()
	reply.Body = fmt.Sprintf("You are now unsubscribed from %s\r\n", listId)
	reply.Send([]string{msg.From})
}