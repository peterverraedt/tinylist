package main

import (
	"fmt"
	"gopkg.in/ini.v1"
	"log"
	"net/smtp"
	"os"
	"strings"
)

type Config struct {
	CommandAddress string `ini:"command_address"`
	Log            string `ini:"log"`
	Database       string `ini:"database"`
	SMTPHostname   string `ini:"smtp_hostname"`
	SMTPPort       string `ini:"smtp_port"`
	SMTPUsername   string `ini:"smtp_username"`
	SMTPPassword   string `ini:"smtp_password"`
	Lists          map[string]*List
	Debug          bool
	ConfigFile     string
}


// Load gConfig from the on-disk config file
func loadConfig() {
	var (
		err error
		cfg *ini.File
	)

	if len(gConfig.ConfigFile) > 0 {
		cfg, err = ini.Load(gConfig.ConfigFile)
	} else {
		cfg, err = ini.LooseLoad("nanolist.ini", "/usr/local/etc/nanolist.ini", "/etc/nanolist.ini")
	}

	if err != nil {
		log.Printf("CONFIG_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}

	err = cfg.Section("").MapTo(gConfig)
	if err != nil {
		log.Printf("CONFIG_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}

	gConfig.Lists = make(map[string]*List)

	for _, section := range cfg.ChildSections("list") {
		list := &List{}
		err = section.MapTo(list)
		if err != nil {
			log.Printf("CONFIG_ERROR Error=%q\n", err.Error())
			os.Exit(0)
		}
		list.Id = strings.TrimPrefix(section.Name(), "list.")
		gConfig.Lists[list.Address] = list
	}
}

// Check for a valid configuration
func checkConfig() bool {
	_, err := openDB()
	if err != nil {
		fmt.Printf("There's a problem with the database: %s\n", err.Error())
		return false
	}

	err = openLog()
	if err != nil {
		fmt.Printf("There's a problem with the log: %s\n", err.Error())
		return false
	}

	client, err := smtp.Dial(gConfig.SMTPHostname + ":" + gConfig.SMTPPort)
	if err != nil {
		fmt.Printf("There's a problem connecting to your SMTP server: %s\n", err.Error())
		return false
	}

	auth := smtp.PlainAuth("", gConfig.SMTPUsername, gConfig.SMTPPassword, gConfig.SMTPHostname)
	err = client.Auth(auth)
	if err != nil {
		fmt.Printf("There's a problem authenticating with your SMTP server: %s\n", err.Error())
		return false
	}

	return true
}

