package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	cli := NewCLI()
	command := cli.Parse(os.Args[1:])

	config := NewConfig(*cli.ConfigFile, *cli.Debug)

	switch command {
	case "check":
		if config.CheckConfig() {
			fmt.Printf("Congratulations, nanolist appears to be successfully set up!")
			os.Exit(0)
		} else {
			os.Exit(1)
		}

	case "message":
		err := config.openLog()
		if err != nil {
			log.Fatalf("LOG_ERROR Error=%q\n", err.Error())
		}

		err = config.Bot.Handle(bufio.NewReader(os.Stdin))
		if err != nil {
			log.Fatalf("ERROR_HANDLING_MESSAGE Error=%q\n", err.Error())
		}

	case "subscribe":
		_, err := config.Bot.Subscribe(*cli.SubscribeAddress, *cli.SubscribeList, true)
		if err != nil {
			log.Fatalf("ERROR_SUBSCRIBING Error=%q\n", err.Error())
		}

	case "unsubscribe":
		_, err := config.Bot.Unsubscribe(*cli.UnsubscribeAddress, *cli.UnsubscribeList, true)
		if err != nil {
			log.Fatalf("ERROR_UNSUBSCRIBING Error=%q\n", err.Error())
		}

	default:
		fmt.Printf("Unknown command %s\n", flag.Arg(0))
	}
}
