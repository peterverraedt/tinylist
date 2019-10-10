package list

import (
	"fmt"
	"io"
	"net/mail"
	"os"

	"github.com/kballard/go-shellquote"
	"gopkg.in/alecthomas/kingpin.v2"
)

// A Command represents a command parser
type Command struct {
	app                *kingpin.Application
	admin              bool
	botFactory         botFactory
	listCmd            *kingpin.CmdClause
	listAll            *bool
	createCmd          *kingpin.CmdClause
	createOptions      *commandListOptions
	modifyCmd          *kingpin.CmdClause
	modifyOptions      *commandListOptions
	deleteCmd          *kingpin.CmdClause
	deleteList         *string
	subscribeCmd       *kingpin.CmdClause
	subscribeOptions   *commandSubscriptionOptions
	unsubscribeCmd     *kingpin.CmdClause
	unsubscribeOptions *commandSubscriptionOptions
	w                  io.Writer
	rc                 *int
}

type commandListOptions struct {
	List        *string
	Name        *string
	Description *string
	Flags       *[]string
	Posters     *[]string
	Bcc         *[]string
}

type commandSubscriptionOptions struct {
	List    *string
	Address *string
}

// NewCommand returns a Command application object
func NewCommand(admin bool, userAddress string, b *bot, w io.Writer) *Command {
	app := kingpin.New("tinylist", "Tiny list server")

	c := AddCommand(app, admin, userAddress, func(*kingpin.ParseContext) *bot {
		return b
	})

	if w != nil {
		c.app.UsageWriter(w)
		c.app.ErrorWriter(w)
		c.w = w
	} else {
		c.w = os.Stdout
	}

	return c
}

// AddCommand adds bot commands to a given kingpin application
func AddCommand(app *kingpin.Application, admin bool, userAddress string, botFactory botFactory) *Command {
	c := &Command{
		app:        app,
		admin:      admin,
		w:          os.Stdout,
		botFactory: botFactory,
	}

	app.PreAction(c.parseAddresses)

	c.listCmd = app.Command("list", "List all lists and their subscribers").Action(c.list)
	c.subscribeCmd = app.Command("subscribe", "Subscribe to a list").Action(c.subscribe)
	c.unsubscribeCmd = app.Command("unsubscribe", "Unsubscribe from a list").Action(c.unsubscribe)

	if admin {
		c.createCmd = app.Command("create", "Create a list").Action(c.create)
		c.modifyCmd = app.Command("modify", "Update a list").Alias("update").Action(c.modify)
		c.deleteCmd = app.Command("delete", "Delete a list").Action(c.delete)

		c.listAll = c.listCmd.Flag("all", "Also list hidden lists").Short('a').Bool()
		c.createOptions = addCommandListOptions(c.createCmd)
		c.modifyOptions = addCommandListOptions(c.modifyCmd)
		c.deleteList = c.deleteCmd.Arg("list", "The list address").Required().String()
	}

	c.subscribeOptions = addCommandSubscriptionOptions(c.subscribeCmd, userAddress, admin, true)
	c.unsubscribeOptions = addCommandSubscriptionOptions(c.unsubscribeCmd, userAddress, admin, false)

	return c
}

func addCommandListOptions(cmd *kingpin.CmdClause) *commandListOptions {
	return &commandListOptions{
		List:        cmd.Arg("list", "The address of the mailing list, must be a valid address pointing to the tinylist pipe").Required().String(),
		Name:        cmd.Flag("name", "The name of the new mailing list, used as a title to refer to this mailing list").String(),
		Description: cmd.Flag("description", "The description of the new mailing list").String(),
		Flags:       cmd.Flag("flag", "Setting flags: locked, hidden, and/or subscribers_only").Short('f').Enums("locked", "hidden", "subscribers_only", ""),
		Posters:     cmd.Flag("poster", "Limit posting on the list to these addresses").Strings(),
		Bcc:         cmd.Flag("bcc", "Always put these addresses in blind copy, useful for archiving").Strings(),
	}
}

func addCommandSubscriptionOptions(cmd *kingpin.CmdClause, userAddress string, admin bool, newSubscription bool) *commandSubscriptionOptions {
	c := &commandSubscriptionOptions{}

	if userAddress == "" && admin {
		c.Address = cmd.Arg("address", "The address used in the subscription").Required().String()
	}

	list := cmd.Arg("list", "The list address")
	if newSubscription {
		list = list.Required()
	}
	c.List = list.String()

	if userAddress != "" {
		if admin {
			c.Address = cmd.Flag("address", "The address used in the subscription").Default(userAddress).Enum(userAddress)
		} else {
			c.Address = cmd.Flag("address", "Override the address used in the subscription").Default(userAddress).String()
		}
	}
	return c
}

func (c *Command) parseAddresses(*kingpin.ParseContext) error {
	addressVars := []*string{
		c.createOptions.List,
		c.modifyOptions.List,
		c.deleteList,
		c.subscribeOptions.List,
		c.subscribeOptions.Address,
		c.unsubscribeOptions.List,
		c.unsubscribeOptions.Address,
	}
	for _, address := range addressVars {
		err := assureAddress(address)
		if err != nil {
			return err
		}
	}

	addressesVars := []*[]string{
		c.createOptions.Posters,
		c.createOptions.Bcc,
		c.modifyOptions.Posters,
		c.modifyOptions.Bcc,
	}
	for _, addresses := range addressesVars {
		err := assureAddresses(addresses)
		if err != nil {
			return err
		}
	}

	return nil
}

func assureAddress(a *string) error {
	if a == nil || *a == "" {
		return nil
	}
	obj, err := mail.ParseAddress(*a)
	if err != nil {
		return err
	}
	a = &obj.Address
	return nil
}

func assureAddresses(a *[]string) error {
	if a == nil {
		return nil
	}
	r := []string{}
	for _, v := range *a {
		if v == "" {
			r = append(r, "")
		} else {
			obj, err := mail.ParseAddress(v)
			if err != nil {
				return err
			}
			r = append(r, obj.Address)
		}
	}
	a = &r
	return nil
}

// Parse parses a CLI using the given arguments
func (c *Command) Parse(params []string) (string, error) {
	return c.app.Parse(params)
}

// ParseString parses a CLI using the given arguments
func (c *Command) ParseString(paramsString string) (string, error) {
	params, err := shellquote.Split(paramsString)
	if err != nil {
		return "", err
	}

	return c.Parse(params)
}

func (c *Command) list(ctx *kingpin.ParseContext) error {
	bot := c.botFactory(ctx)

	lists, err := bot.Lists()
	if err != nil {
		return fmt.Errorf("Retrieving lists failed with error: %s", err.Error())
	}

	fmt.Fprintf(c.w, "Available mailing lists:\n\n")
	for _, list := range lists {
		if !list.Hidden || *c.listAll {
			if c.admin {
				fmt.Fprintf(c.w, "%s\n\n", list.String())
			} else {
				fmt.Fprintf(c.w, "%s <%s>: %s\n", list.Name, list.Address, list.Description)
			}
		}
	}
	fmt.Fprintf(c.w, "\nTo subscribe to a mailing list, email %s with 'subscribe <list-address>' as the subject.\n", bot.CommandAddress)

	return nil
}

func (c *Command) create(ctx *kingpin.ParseContext) error {
	bot := c.botFactory(ctx)

	list, err := bot.LookupList(*c.createOptions.List)
	if err != nil {
		return err
	}
	if list != nil {
		return fmt.Errorf("List %s already exists", list.Address)
	}

	d := Definition{
		Address:     *c.createOptions.List,
		Name:        *c.createOptions.Name,
		Description: *c.createOptions.Description,
	}

	for _, flag := range *c.createOptions.Flags {
		switch flag {
		case "hidden":
			d.Hidden = true
		case "locked":
			d.Locked = true
		case "subscribersOnly":
			d.SubscribersOnly = true
		}
	}

	d.Posters = []string{}
	for _, address := range *c.modifyOptions.Posters {
		if address != "" {
			d.Posters = append(d.Posters, address)
		}
	}
	d.Bcc = []string{}
	for _, address := range *c.modifyOptions.Bcc {
		if address != "" {
			d.Bcc = append(d.Bcc, address)
		}
	}

	err = bot.CreateList(d)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.w, "List %s has successfully been created.\n\n%s\n", d.Address, d.String())
	return nil
}

func (c *Command) modify(ctx *kingpin.ParseContext) error {
	bot := c.botFactory(ctx)

	list, err := bot.LookupList(*c.modifyOptions.List)
	if err != nil {
		return err
	}
	if list == nil {
		return fmt.Errorf("List %s does not exist", *c.modifyOptions.List)
	}

	d := Definition{
		Address: *c.modifyOptions.List,
	}

	if *c.modifyOptions.Name != "" {
		d.Name = *c.modifyOptions.Name
	} else {
		d.Name = list.Name
	}
	if *c.modifyOptions.Description != "" {
		d.Description = *c.modifyOptions.Description
	} else {
		d.Description = list.Description
	}
	if len(*c.modifyOptions.Posters) > 0 {
		d.Posters = []string{}
		for _, address := range *c.modifyOptions.Posters {
			if address != "" {
				d.Posters = append(d.Posters, address)
			}
		}
	} else {
		d.Posters = list.Posters
	}
	if len(*c.modifyOptions.Bcc) > 0 {
		d.Bcc = []string{}
		for _, address := range *c.modifyOptions.Bcc {
			if address != "" {
				d.Bcc = append(d.Bcc, address)
			}
		}
	} else {
		d.Bcc = list.Bcc
	}

	if len(*c.modifyOptions.Flags) > 0 {
		for _, flag := range *c.modifyOptions.Flags {
			switch flag {
			case "hidden":
				d.Hidden = true
			case "locked":
				d.Locked = true
			case "subscribersOnly":
				d.SubscribersOnly = true
			}
		}
	} else {
		d.Hidden = list.Hidden
		d.Locked = list.Locked
		d.SubscribersOnly = list.SubscribersOnly
	}

	err = bot.ModifyList(list, d)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.w, "List %s has successfully been updated.\n\n%s\n", list.Address, d.String())
	return nil
}

func (c *Command) delete(ctx *kingpin.ParseContext) error {
	bot := c.botFactory(ctx)

	list, err := bot.LookupList(*c.deleteList)
	if err != nil {
		return err
	}
	if list == nil {
		return fmt.Errorf("List %s does not exist", *c.deleteList)
	}

	err = bot.DeleteList(list)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.w, "List %s has successfully been deleted.\n", list.Address)
	return nil
}

func (c *Command) subscribe(ctx *kingpin.ParseContext) error {
	bot := c.botFactory(ctx)

	list, err := bot.Subscribe(*c.subscribeOptions.Address, *c.subscribeOptions.List, c.admin)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.w, "You are now subscribed to %s.\n", list.Address)
	return nil
}

func (c *Command) unsubscribe(ctx *kingpin.ParseContext) error {
	bot := c.botFactory(ctx)

	if *c.unsubscribeOptions.List == "" {
		lists, err := bot.UnsubscribeAll(*c.unsubscribeOptions.Address, c.admin)
		if err != nil {
			return err
		}
		for _, list := range lists {
			fmt.Fprintf(c.w, "You are now unsubscribed from %s.\n", list.Address)
		}
		return nil
	}
	list, err := bot.Unsubscribe(*c.unsubscribeOptions.Address, *c.unsubscribeOptions.List, c.admin)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.w, "You are now unsubscribed from %s.\n", list.Address)
	return nil
}
