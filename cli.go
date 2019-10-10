package main

import (
	"log"
	"net/mail"

	"gopkg.in/alecthomas/kingpin.v2"
)

// A CLI variable collection object
type CLI struct {
	app                *kingpin.Application
	Debug              *bool
	ConfigFile         *string
	Check              *kingpin.CmdClause
	Message            *kingpin.CmdClause
	List               *kingpin.CmdClause
	Create             *kingpin.CmdClause
	CreateOptions      *CLIListOptions
	Modify             *kingpin.CmdClause
	ModifyOptions      *CLIListOptions
	Delete             *kingpin.CmdClause
	DeleteList         *string
	Subscribe          *kingpin.CmdClause
	SubscribeOptions   *CLISubscriptionOptions
	Unsubscribe        *kingpin.CmdClause
	UnsubscribeOptions *CLISubscriptionOptions
}

// CLIListOptions represent the options during creation/updates of a list
type CLIListOptions struct {
	List        *string
	Name        *string
	Description *string
	Flags       *[]string
	Posters     *[]string
	Bcc         *[]string
}

// CLISubscriptionOptions represent the options during (un)subscriptions
type CLISubscriptionOptions struct {
	List    *string
	Address *string
}

// NewCLI returns a CLI object
func NewCLI() *CLI {
	app := kingpin.New("nanolist", "Nano list server")
	app.HelpFlag.Short('h')
	c := &CLI{
		app:         app,
		Debug:       app.Flag("debug", "Don't send emails - print them to stdout instead").Bool(),
		ConfigFile:  app.Flag("config", "Load configuration from specified file").Default("").String(),
		Check:       app.Command("check", "Check the configuration"),
		Message:     app.Command("message", "Process a message from stdin"),
		List:        app.Command("list", "List all lists and their subscribers"),
		Create:      app.Command("create", "Create a list"),
		Modify:      app.Command("modify", "Update a list").Alias("update"),
		Delete:      app.Command("delete", "Delete a list"),
		Subscribe:   app.Command("subscribe", "Subscribe an address on a list"),
		Unsubscribe: app.Command("unsubscribe", "Unsubscribe an address from a list"),
	}

	c.CreateOptions = addCLIListOptions(c.Create)
	c.ModifyOptions = addCLIListOptions(c.Modify)
	c.DeleteList = c.Delete.Arg("list", "The list address").Required().String()

	c.SubscribeOptions = addCLISubscriptionOptions(c.Subscribe, true)
	c.UnsubscribeOptions = addCLISubscriptionOptions(c.Unsubscribe, false)

	return c
}

func addCLIListOptions(cmd *kingpin.CmdClause) *CLIListOptions {
	return &CLIListOptions{
		List:        cmd.Arg("list", "The address of the mailing list, must be a valid address pointing to the nanolist pipe").Required().String(),
		Name:        cmd.Flag("name", "The name of the new mailing list, used as a title to refer to this mailing list").String(),
		Description: cmd.Flag("description", "The description of the new mailing list").String(),
		Flags:       cmd.Flag("flag", "Setting flags: locked, hidden, and/or subscribers_only").Short('f').Enums("locked", "hidden", "subscribers_only", ""),
		Posters:     cmd.Flag("poster", "Limit posting on the list to these addresses").Strings(),
		Bcc:         cmd.Flag("bcc", "Always put these addresses in blind copy, useful for archiving").Strings(),
	}
}

func addCLISubscriptionOptions(cmd *kingpin.CmdClause, require bool) *CLISubscriptionOptions {
	c := &CLISubscriptionOptions{
		Address: cmd.Arg("address", "The address used in the subscription").Required().String(),
	}
	list := cmd.Arg("list", "The list address")
	if require {
		list = list.Required()
	}
	c.List = list.String()
	return c
}

// Parse parses a CLI using the given arguments
func (c *CLI) Parse(params []string) string {
	command, err := c.app.Parse(params)
	if err != nil {
		log.Fatal(err)
	}
	assureAddress(c.CreateOptions.List)
	assureAddresses(c.CreateOptions.Posters)
	assureAddresses(c.CreateOptions.Bcc)
	assureAddress(c.ModifyOptions.List)
	assureAddresses(c.ModifyOptions.Posters)
	assureAddresses(c.ModifyOptions.Bcc)
	assureAddress(c.DeleteList)
	assureAddress(c.SubscribeOptions.List)
	assureAddress(c.SubscribeOptions.Address)
	assureAddress(c.UnsubscribeOptions.List)
	assureAddress(c.UnsubscribeOptions.Address)
	return command
}

func assureAddress(a *string) {
	if a == nil || *a == "" {
		return
	}
	obj, err := mail.ParseAddress(*a)
	if err != nil {
		log.Fatal(err)
	}
	a = &obj.Address
}

func assureAddresses(a *[]string) {
	if a == nil {
		return
	}
	r := []string{}
	for _, v := range *a {
		if v == "" {
			r = append(r, "")
		} else {
			obj, err := mail.ParseAddress(v)
			if err != nil {
				log.Fatal(err)
			}
			r = append(r, obj.Address)
		}
	}
	a = &r
}
