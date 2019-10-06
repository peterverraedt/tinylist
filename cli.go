package main

import (
	"log"

	"gopkg.in/alecthomas/kingpin.v2"
)

type CLI struct {
	app                *kingpin.Application
	Debug              *bool
	ConfigFile         *string
	Check              *kingpin.CmdClause
	Message            *kingpin.CmdClause
	Subscribe          *kingpin.CmdClause
	SubscribeList      *string
	SubscribeAddress   *string
	Unsubscribe        *kingpin.CmdClause
	UnsubscribeList    *string
	UnsubscribeAddress *string
}

func NewCLI() *CLI {
	app := kingpin.New("nanolist", "Nano list server")
	app.HelpFlag.Short('h')
	c := &CLI{
		app:         app,
		Debug:       app.Flag("debug", "Don't send emails - print them to stdout instead").Bool(),
		ConfigFile:  app.Flag("config", "Load configuration from specified file").Default("").String(),
		Check:       app.Command("check", "Check the configuration"),
		Message:     app.Command("message", "Process a message from stdin"),
		Subscribe:   app.Command("subscribe", "Subscribe an address on a list"),
		Unsubscribe: app.Command("unsubscribe", "Unsubscribe an address from a list"),
	}

	c.SubscribeList = c.Subscribe.Flag("list", "The list for which the subscription should be modified").Required().String()
	c.SubscribeAddress = c.Subscribe.Flag("address", "The e-mail address of the subscription").Required().String()

	c.UnsubscribeList = c.Unsubscribe.Flag("list", "The list for which the subscription should be modified").Required().String()
	c.UnsubscribeAddress = c.Unsubscribe.Flag("address", "The e-mail address of the subscription").Required().String()

	return c
}

func (c *CLI) Parse(params []string) string {
	command, err := c.app.Parse(params)
	if err != nil {
		log.Fatal(err)
	}
	return command
}
