package main

import (
	"log"
	"os"
)

func main() {
	cli := NewCLI()
	_, err := cli.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	/*
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
			handleErr(config.Delete(*cli.DeleteList))*/
}
