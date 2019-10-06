package main

import (
	"fmt"
	"io"
	"log"
	"net/smtp"
	"os"
	"strings"

	"gopkg.in/ini.v1"

	"github.com/peter.verraedt/nanolist/list"
)

// Config
type Config struct {
	Log      string `ini:"log"`
	Database string `ini:"database"`
	Bot      *list.Bot
}

// NewConfig from the on-disk config file
func NewConfig(configFile string, debug bool) *Config {
	var (
		err error
		cfg *ini.File
	)

	if len(configFile) > 0 {
		cfg, err = ini.Load(configFile)
	} else {
		cfg, err = ini.LooseLoad("nanolist.ini", "/usr/local/etc/nanolist.ini", "/etc/nanolist.ini")
	}

	if err != nil {
		log.Fatalf("CONFIG_ERROR Error=%q\n", err.Error())
	}

	c := &Config{}
	err = cfg.Section("").MapTo(c)
	if err != nil {
		log.Fatalf("CONFIG_ERROR Error=%q\n", err.Error())
	}

	c.Bot = &list.Bot{}
	err = cfg.Section("bot").MapTo(c.Bot)
	if err != nil {
		log.Fatalf("CONFIG_ERROR Error=%q\n", err.Error())
	}
	if debug {
		c.Bot.Debug = true
	}

	c.Bot.Lists = make(map[string]*list.List)

	for _, section := range cfg.ChildSections("list") {
		list := &list.List{}
		err = section.MapTo(list)
		if err != nil {
			log.Fatalf("CONFIG_ERROR Error=%q\n", err.Error())
		}
		list.Id = strings.TrimPrefix(section.Name(), "list.")
		list.Subscribe = func(address string) error { return c.addSubscription(address, list.Id) }
		list.Unsubscribe = func(address string) error { return c.removeSubscription(address, list.Id) }
		list.Subscribers = func() ([]string, error) { return c.fetchSubscribers(list.Id) }
		list.IsSubscribed = func(address string) (bool, error) { return c.isSubscribed(address, list.Id) }
		c.Bot.Lists[list.Address] = list
	}

	return c
}

// CheckConfig checks for a valid configuration
func (c *Config) CheckConfig() bool {
	_, err := c.openDB()
	if err != nil {
		fmt.Printf("There's a problem with the database: %s\n", err.Error())
		return false
	}

	err = c.openLog()
	if err != nil {
		fmt.Printf("There's a problem with the log: %s\n", err.Error())
		return false
	}

	client, err := smtp.Dial(fmt.Sprintf("%s:%d", c.Bot.SMTPHostname, c.Bot.SMTPPort))
	if err != nil {
		fmt.Printf("There's a problem connecting to your SMTP server: %s\n", err.Error())
		return false
	}

	if c.Bot.SMTPUsername != "" {
		auth := smtp.PlainAuth("", c.Bot.SMTPUsername, c.Bot.SMTPPassword, c.Bot.SMTPHostname)
		err = client.Auth(auth)
		if err != nil {
			fmt.Printf("There's a problem authenticating with your SMTP server: %s\n", err.Error())
			return false
		}
	}

	return true
}

func (c *Config) openLog() error {
	logFile, err := os.OpenFile(c.Log, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	out := io.MultiWriter(logFile, os.Stderr)
	log.SetOutput(out)
	return nil
}
