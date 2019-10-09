package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/peterverraedt/nanolist/list"
)

// Open the database
func (c *Config) openDB() (err error) {
	c.db, err = sql.Open("sqlite3", c.Database)

	if err != nil {
		return
	}

	_, err = c.db.Exec(`
	CREATE TABLE IF NOT EXISTS "lists" (
		"list" TEXT PRIMARY KEY,
		"name" TEXT NOT NULL,
		"description" TEXT NOT NULL,
		"hidden" INTEGER(1) NOT NULL,
		"locked" INTEGER(1) NOT NULL,
		"subscribers_only" INTEGER(1) NOT NULL
	);
	CREATE TABLE IF NOT EXISTS "bcc" (
		"list" TEXT NOT NULL,
		"address" TEXT NOT NULL,
		UNIQUE("list","address")
	);
	CREATE TABLE IF NOT EXISTS "posters" (
		"list" TEXT NOT NULL,
		"address" TEXT NOT NULL,
		UNIQUE("list","address")
	);
	CREATE TABLE IF NOT EXISTS "subscriptions" (
		"list" TEXT NOT NULL,
		"user" TEXT NOT NULL,
		"bounces" INTEGER NOT NULL DEFAULT 0,
		"last_bounce" DATETIME NOT NULL DEFAULT 0,
		UNIQUE("list","user")
	);
	`)

	return
}

// Lists returns all lists
func (c *Config) Lists() ([]*list.List, error) {
	rows, err := c.db.Query("SELECT list, name, description, hidden, locked, subscribers_only FROM lists ORDER BY list")
	if err != nil {
		return nil, err
	}

	result := []*list.List{}
	defer rows.Close()
	for rows.Next() {
		l, err := c.fetchList(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, l)
	}

	return result, nil
}

// LookupLists returns a specific list, or nil if not found
func (c *Config) LookupList(name string) (*list.List, error) {
	rows, err := c.db.Query("SELECT list, name, description, hidden, locked, subscribers_only FROM lists WHERE list=?", name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	if rows.Next() {
		l, err := c.fetchList(rows)
		if err != nil {
			return nil, err
		}

		if rows.Next() {
			return nil, nil
		}

		return l, nil
	}

	return nil, nil
}

func (c *Config) fetchList(rows *sql.Rows) (*list.List, error) {
	l := &list.List{}
	err := rows.Scan(&l.ID, &l.Name, &l.Description, &l.Hidden, &l.Locked, &l.SubscribersOnly)
	if err != nil {
		return nil, err
	}
	l.Posters, err = c.listPosters(l.ID)
	if err != nil {
		return nil, err
	}
	l.Bcc, err = c.listBcc(l.ID)
	if err != nil {
		return nil, err
	}
	l.Subscribe = func(a string) error { return c.subscribe(l.ID, a) }
	l.Unsubscribe = func(a string) error { return c.unsubscribe(l.ID, a) }
	l.SetBounce = func(a string, b uint16, t time.Time) error { return c.setBounce(l.ID, a, b, t) }
	l.Subscribers = func() ([]*list.Subscription, error) { return c.listSubscribers(l.ID) }
	l.IsSubscribed = func(a string) (*list.Subscription, error) { return c.isSubscribed(l.ID, a) }
	return l, nil
}

func (c *Config) listPosters(id string) ([]string, error) {
	rows, err := c.db.Query("SELECT address FROM posters WHERE list=?", id)
	if err != nil {
		return nil, err
	}

	result := []string{}
	defer rows.Close()
	for rows.Next() {
		var address string
		err = rows.Scan(&address)
		if err != nil {
			return nil, err
		}
		result = append(result, address)
	}

	return result, nil
}

func (c *Config) listBcc(id string) ([]string, error) {
	rows, err := c.db.Query("SELECT address FROM bcc WHERE list=?", id)
	if err != nil {
		return nil, err
	}

	result := []string{}
	defer rows.Close()
	for rows.Next() {
		var address string
		err = rows.Scan(&address)
		if err != nil {
			return nil, err
		}
		result = append(result, address)
	}

	return result, nil
}

func (c *Config) isSubscribed(id string, user string) (*list.Subscription, error) {
	s := &list.Subscription{
		Address: user,
	}
	err := c.db.QueryRow("SELECT bounces, last_bounce FROM subscriptions WHERE user=? AND list=?", user, id).Scan(&s.Bounces, &s.LastBounce)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return s, nil
}

func (c *Config) listSubscribers(id string) ([]*list.Subscription, error) {
	rows, err := c.db.Query("SELECT user, bounces, last_bounce FROM subscriptions WHERE list=?", id)
	if err != nil {
		return nil, err
	}

	result := []*list.Subscription{}
	defer rows.Close()
	for rows.Next() {
		s := &list.Subscription{}
		err = rows.Scan(&s.Address, &s.Bounces, &s.LastBounce)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}

	return result, nil
}

func (c *Config) subscribe(id string, user string) error {
	_, err := c.db.Exec("INSERT INTO subscriptions (user,list) VALUES(?,?)", user, id)
	return err
}

func (c *Config) unsubscribe(id string, user string) error {
	r, err := c.db.Exec("DELETE FROM subscriptions WHERE user=? AND list=?", user, id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("User %s is not subscribed to list %s", user, id)
	}
	return nil
}

func (c *Config) setBounce(id string, user string, bounces uint16, lastBounce time.Time) error {
	r, err := c.db.Exec("UPDATE subscriptions SET bounces = ?, last_bounce = ? WHERE user=? AND list=?", bounces, lastBounce, user, id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("User %s is not subscribed to list %s", user, id)
	}
	return nil
}

// Create a list
func (c *Config) Create(o *CLIListOptions) error {
	var hidden, locked, subscribersOnly bool
	for _, flag := range *o.Flags {
		switch flag {
		case "hidden":
			hidden = true
		case "locked":
			locked = true
		case "subscribersOnly":
			subscribersOnly = true
		}
	}

	tx, _ := c.db.Begin()

	_, err := tx.Exec("INSERT INTO lists (list, name, description, hidden, locked, subscribers_only) VALUES(?,?,?,?,?,?)",
		*o.List, *o.Name, *o.Description, hidden, locked, subscribersOnly)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, address := range *o.Posters {
		if address == "" {
			continue
		}

		_, err = tx.Exec("INSERT INTO posters (list, address) VALUES(?,?)", o.List, address)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, address := range *o.Bcc {
		if address == "" {
			continue
		}

		_, err = tx.Exec("INSERT INTO bcc (list, address) VALUES(?,?)", o.List, address)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	tx.Commit()
	return nil
}

// Modify a list
func (c *Config) Modify(o *CLIListOptions) error {
	tx, _ := c.db.Begin()

	exists := false
	err := c.db.QueryRow("SELECT 1 FROM lists WHERE list=?", *o.List).Scan(&exists)

	if err != nil {
		tx.Rollback()
		return err
	}

	if *o.Name != "" {
		_, err = tx.Exec("UPDATE lists SET name = ? WHERE list = ?", *o.Name, *o.List)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	if *o.Description != "" {
		_, err = tx.Exec("UPDATE lists SET description = ? WHERE list = ?", *o.Description, *o.List)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if len(*o.Flags) > 0 {
		var hidden, locked, subscribersOnly bool
		for _, flag := range *o.Flags {
			switch flag {
			case "hidden":
				hidden = true
			case "locked":
				locked = true
			case "subscribers_only":
				subscribersOnly = true
			}
		}

		_, err = tx.Exec("UPDATE lists SET hidden = ?, locked = ?, subscribers_only = ? WHERE list = ?", hidden, locked, subscribersOnly, *o.List)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if len(*o.Posters) > 0 {
		_, err = tx.Exec("DELETE FROM posters WHERE list = ?", *o.List)
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, address := range *o.Posters {
			if address == "" {
				continue
			}

			_, err = tx.Exec("INSERT INTO posters (list, address) VALUES(?,?)", *o.List, address)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	if len(*o.Bcc) > 0 {
		_, err = tx.Exec("DELETE FROM bcc WHERE list = ?", *o.List)
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, address := range *o.Bcc {
			if address == "" {
				continue
			}

			_, err = tx.Exec("INSERT INTO bcc (list, address) VALUES(?,?)", *o.List, address)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	tx.Commit()
	return nil
}

// Delete a list
func (c *Config) Delete(id string) error {
	tx, _ := c.db.Begin()

	_, err := tx.Exec("DELETE FROM subscriptions WHERE list = ?", id)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM bcc WHERE list = ?", id)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM posters WHERE list = ?", id)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM lists WHERE list = ?", id)
	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}
