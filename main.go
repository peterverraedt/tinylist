package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
)

var gConfig *Config

// Entry point

func main() {
	gConfig = &Config{}

	flag.BoolVar(&gConfig.Debug, "debug", false, "Don't send emails - print them to stdout instead")
	flag.StringVar(&gConfig.ConfigFile, "config", "", "Load configuration from specified file")
	flag.Parse()

	loadConfig()

	if len(flag.Args()) < 1 {
		fmt.Printf("Error: Command not specified\n")
		os.Exit(1)
	}

	if flag.Arg(0) == "check" {
		if checkConfig() {
			fmt.Printf("Congratulations, nanolist appears to be successfully set up!")
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}

	requireLog()

	if flag.Arg(0) == "message" {
		msg := &Message{}
		err := msg.FromReader(bufio.NewReader(os.Stdin))
		if err != nil {
			log.Printf("ERROR_PARSING_MESSAGE Error=%q\n", err.Error())
			os.Exit(0)
		}
		log.Printf("MESSAGE_RECEIVED Id=%q From=%q To=%q Cc=%q Bcc=%q Subject=%q\n",
			msg.Id, msg.From, msg.To, msg.Cc, msg.Bcc, msg.Subject)
		handleMessage(msg)
	} else {
		fmt.Printf("Unknown command %s\n", flag.Arg(0))
	}
}

// Figure out if this is a command, or a mailing list post
func handleMessage(msg *Message) {
	if isToCommandAddress(msg) {
		handleCommand(msg)
	} else {
		lists := lookupLists(msg)
		if len(lists) > 0 {
			for _, list := range lists {
				if list.CanPost(msg.From) {
					listMsg := msg.ResendAs(list.Id, list.Address)
					list.Send(listMsg)
					log.Printf("MESSAGE_SENT ListId=%q Id=%q From=%q To=%q Cc=%q Bcc=%q Subject=%q\n",
						list.Id, listMsg.Id, listMsg.From, listMsg.To, listMsg.Cc, listMsg.Bcc, listMsg.Subject)
				} else {
					handleNotAuthorisedToPost(msg)
				}
			}
		} else {
			handleNoDestination(msg)
		}
	}
}