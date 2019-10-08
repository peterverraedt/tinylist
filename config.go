package main

import (
	"database/sql"
	"fmt"
	"gopkg.in/ini.v1"
	"io"
	"log"
	"net/smtp"
	"os"

	"github.com/peterverraedt/nanolist/list"
)

// Config
type Config struct {
	Log      string `ini:"log"`
	Database string `ini:"database"`
	Bot      *list.Bot
	db       *sql.DB
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
		log.Fatalf("CONFIG_ERROR [bot] Error=%q\n", err.Error())
	}
	if debug {
		c.Bot.Debug = true
	}

	err = c.openDB()
	if err != nil {
		log.Fatalf("CONFIG_ERROR [db] Error=%q\n", err.Error())
	}

	c.Bot.Lists = c.Lists
	c.Bot.LookupList = c.LookupList

	return c
}

// CheckConfig checks for a valid configuration
func (c *Config) CheckConfig() bool {
	err := c.openLog()
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
