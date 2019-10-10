package main

import (
	"gopkg.in/alecthomas/kingpin.v2"
	"log"
	"os"

	"github.com/peterverraedt/nanolist/list"
)

func main() {
	err := run(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}

func run(params []string) error {
	backend := NewSQLBackend()

	app := kingpin.New("nanolist", "Nano list server")
	app.HelpFlag.Short('h')
	debug := app.Flag("debug", "Don't send emails - print them to stdout instead").Bool()
	configFile := app.Flag("config", "Load configuration from specified file").Default("").String()

	app.Command("check", "Check the configuration").Action(backend.check)
	app.Command("message", "Process a message from stdin").Action(backend.message)
	list.AddCommand(app, true, "", list.NewBotFactory(backend))

	app.Action(func(*kingpin.ParseContext) error {
		return backend.LoadConfig(*configFile, *debug)
	})

	_, err := app.Parse(params)
	return err
}
