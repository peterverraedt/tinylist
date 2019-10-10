package main

import (
	"bufio"
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/peterverraedt/nanolist/list"
)

// A CLI variable collection object expanding *list.Command
type CLI struct {
	app        *kingpin.Application
	Debug      *bool
	ConfigFile *string
}

// NewCLI returns a CLI object
func NewCLI() *CLI {
	app := kingpin.New("nanolist", "Nano list server")
	app.HelpFlag.Short('h')

	c := &CLI{
		app:        app,
		Debug:      app.Flag("debug", "Don't send emails - print them to stdout instead").Bool(),
		ConfigFile: app.Flag("config", "Load configuration from specified file").Default("").String(),
	}

	app.Command("check", "Check the configuration").Action(c.check)
	app.Command("message", "Process a message from stdin").Action(c.message)

	list.AddCommand(app, true, "", c.initializeBot)

	return c
}

// Parse parses a CLI using the given arguments
func (c *CLI) Parse(params []string) (string, error) {
	return c.app.Parse(params)
}

func (c *CLI) initializeBot(*kingpin.ParseContext) (*list.Bot, error) {
	config := NewConfig(*c.ConfigFile, *c.Debug)

	return config.Bot, nil
}

func (c *CLI) check(*kingpin.ParseContext) error {
	config := NewConfig(*c.ConfigFile, *c.Debug)

	err := config.CheckConfig()
	if err != nil {
		return err
	}
	fmt.Println("Congratulations, nanolist appears to be successfully set up!")

	return nil
}

func (c *CLI) message(*kingpin.ParseContext) error {
	config := NewConfig(*c.ConfigFile, *c.Debug)

	err := config.openLog()
	if err != nil {
		return err
	}
	return config.Bot.Handle(bufio.NewReader(os.Stdin))
}
