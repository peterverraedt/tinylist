package main

import (
	"database/sql"
	"log"
	"net/mail"

	_ "github.com/mattn/go-sqlite3"
)

// Open the database
func (c *Config) openDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", c.Database)

	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS "lists" (
		"list" TEXT PRIMARY KEY,
		"name" TEXT,
		"description" TEXT,
		"address" TEXT,
		"hidden" INTEGER(1),
		"locked" INTEGER(1),
		"subscribers_only" INTEGER(1),
	);
	CREATE TABLE IF NOT EXISTS "subscriptions" (
		"list" TEXT,
		"user" TEXT,
		UNIQUE("list","user"),
	);
	`)

	return db, err
}

// Fetch list of subscribers to a mailing list from database
func (c *Config) fetchSubscribers(listId string) ([]string, error) {
	db, err := c.openDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT user FROM subscriptions WHERE list=?", listId)
	if err != nil {
		return nil, err
	}

	listIds := []string{}
	defer rows.Close()
	for rows.Next() {
		var user string
		rows.Scan(&user)
		listIds = append(listIds, user)
	}

	return listIds, nil
}

// Check if a user is subscribed to a mailing list
func (c *Config) isSubscribed(user string, list string) (bool, error) {
	addressObj, err := mail.ParseAddress(user)
	if err != nil {
		return false, err
	}
	db, err := c.openDB()
	if err != nil {
		return false, err
	}

	exists := false
	err = db.QueryRow("SELECT 1 FROM subscriptions WHERE user=? AND list=?", addressObj.Address, list).Scan(&exists)

	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// Add a subscription to the subscription database
func (c *Config) addSubscription(user string, list string) error {
	addressObj, err := mail.ParseAddress(user)
	if err != nil {
		return err
	}

	db, err := c.openDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO subscriptions (user,list) VALUES(?,?)", addressObj.Address, list)
	if err != nil {
		return err
	}
	log.Printf("SUBSCRIPTION_ADDED User=%q List=%q\n", user, list)
	return nil
}

// Remove a subscription from the subscription database
func (c *Config) removeSubscription(user string, list string) error {
	addressObj, err := mail.ParseAddress(user)
	if err != nil {
		return err
	}

	db, err := c.openDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM subscriptions WHERE user=? AND list=?", addressObj.Address, list)
	if err != nil {
		return err
	}
	log.Printf("SUBSCRIPTION_REMOVED User=%q List=%q\n", user, list)
	return nil
}

// Remove all subscriptions from a given mailing list
func (c *Config) clearSubscriptions(list string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM subscriptions WHERE AND list=?", list)
	return err
}
