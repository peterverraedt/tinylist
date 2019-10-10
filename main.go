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
		err := config.CheckConfig()
		if err != nil {
			fmt.Println("Congratulations, nanolist appears to be successfully set up!")
		} else {
			log.Fatal(err)
		}

	case "list":
		lists, err := config.Bot.Lists()
		handleErr(err)

		for _, list := range lists {
			fmt.Printf("%s\n", list)
		}

	case "create":
		handleErr(config.Create(cli.CreateOptions))
		list, _ := config.LookupList(*cli.CreateOptions.List)
		if list != nil {
			fmt.Printf("%s\n", list)
		}

	case "modify":
		handleErr(config.Modify(cli.ModifyOptions))
		list, _ := config.LookupList(*cli.ModifyOptions.List)
		if list != nil {
			fmt.Printf("%s\n", list)
		}

	case "delete":
		handleErr(config.Delete(*cli.DeleteList))

	case "subscribe":
		_, err := config.Bot.Subscribe(*cli.SubscribeOptions.Address, *cli.SubscribeOptions.List, true)
		handleErr(err)

	case "unsubscribe":
		if *cli.UnsubscribeOptions.List == "" {
			_, err := config.Bot.UnsubscribeAll(*cli.UnsubscribeOptions.Address, true)
			handleErr(err)
		} else {
			_, err := config.Bot.Unsubscribe(*cli.UnsubscribeOptions.Address, *cli.UnsubscribeOptions.List, true)
			handleErr(err)
		}

	case "message":
		handleErr(config.openLog())
		handleErr(config.Bot.Handle(bufio.NewReader(os.Stdin)))

	default:
		log.Fatalf("Unknown command %s\n", flag.Arg(0))
	}
}

func handleErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
