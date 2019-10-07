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
	Create             *kingpin.CmdClause
	CreateList         *string
	CreateAddress      *string
	CreateName         *string
	Modify             *kingpin.CmdClause
	ModifyList         *string
	ModifyAddress      *string
	ModifyName         *string
	Delete             *kingpin.CmdClause
	DeleteList         *string
	Lock               *kingpin.CmdClause
	Unlock             *kingpin.CmdClause
	Description        *kingpin.CmdClause
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
		Create:      app.Command("create", "Create a list"),
		Modify:      app.Command("update", "Update a list"),
		Delete:      app.Command("delete", "Delete a list"),
		Lock:        app.Command("lock", "Lock a list"),
		Unlock:      app.Command("unlock", "Lock a list"),
		Description: app.Command("set-description", "Set a list description"),
		Subscribe:   app.Command("subscribe", "Subscribe an address on a list"),
		Unsubscribe: app.Command("unsubscribe", "Unsubscribe an address from a list"),
	}

	c.CreateList = c.Create.Flag("list", "The list ID of the new mailing list").Required().String()
	c.CreateAddress = c.Create.Flag("address", "The address of the new mailing list, must be a valid address pointing to the nanolist pipe").Required().String()
	c.CreateName = c.Create.Flag("name", "The name of the new mailing list, used as a title to refer to this mailing list").Required().String()

	c.ModifyList = c.Modify.Flag("list", "The list ID to modify").Required().String()
	c.ModifyAddress = c.Modify.Flag("address", "The address of the new mailing list, must be a valid address pointing to the nanolist pipe").String()
	c.ModifyName = c.Modify.Flag("name", "The name of the new mailing list, used as a title to refer to this mailing list").String()

	c.DeleteList = c.Create.Flag("list", "The list ID to remove").Required().String()

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
