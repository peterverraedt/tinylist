package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/mail"
	"os"
)


// Open the database
func openDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", gConfig.Database)

	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS "subscriptions" (
		"list" TEXT,
		"user" TEXT
	);
	`)

	return db, err
}

// Open the database or fail immediately
func requireDB() *sql.DB {
	db, err := openDB()
	if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(1)
	}
	return db
}



// Fetch list of subscribers to a mailing list from database
func fetchSubscribers(listId string) []string {
	db := requireDB()
	rows, err := db.Query("SELECT user FROM subscriptions WHERE list=?", listId)

	if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}

	listIds := []string{}
	defer rows.Close()
	for rows.Next() {
		var user string
		rows.Scan(&user)
		listIds = append(listIds, user)
	}

	return listIds
}

// Check if a user is subscribed to a mailing list
func isSubscribed(user string, list string) bool {
	addressObj, err := mail.ParseAddress(user)
	if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}
	db := requireDB()

	exists := false
	err = db.QueryRow("SELECT 1 FROM subscriptions WHERE user=? AND list=?", addressObj.Address, list).Scan(&exists)

	if err == sql.ErrNoRows {
		return false
	} else if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}

	return true
}

// Add a subscription to the subscription database
func addSubscription(user string, list string) {
	addressObj, err := mail.ParseAddress(user)
	if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}

	db := requireDB()
	_, err = db.Exec("INSERT INTO subscriptions (user,list) VALUES(?,?)", addressObj.Address, list)
	if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}
	log.Printf("SUBSCRIPTION_ADDED User=%q List=%q\n", user, list)
}

// Remove a subscription from the subscription database
func removeSubscription(user string, list string) {
	addressObj, err := mail.ParseAddress(user)
	if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}

	db := requireDB()
	_, err = db.Exec("DELETE FROM subscriptions WHERE user=? AND list=?", addressObj.Address, list)
	if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}
	log.Printf("SUBSCRIPTION_REMOVED User=%q List=%q\n", user, list)
}

// Remove all subscriptions from a given mailing list
func clearSubscriptions(list string) {
	db := requireDB()
	_, err := db.Exec("DELETE FROM subscriptions WHERE AND list=?", list)
	if err != nil {
		log.Printf("DATABASE_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}
}