package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/smtp"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/peterverraedt/nanolist/list"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/ini.v1"
)

// An SQLBackend is an implementation of list.Backend
type SQLBackend struct {
	Log      string `ini:"log"`
	Database string `ini:"database"`
	config   list.Config
	db       *sql.DB
}

// NewSQLBackend from the on-disk config file
func NewSQLBackend() *SQLBackend {
	return &SQLBackend{}
}

// LoadConfig loads a config file and needs to be called before using it as a Backend interface
func (b *SQLBackend) LoadConfig(configFile string, debug bool) error {
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
		return fmt.Errorf("Config ini error: %s", err.Error())
	}

	err = cfg.Section("").MapTo(b)
	if err != nil {
		return fmt.Errorf("Config parse error (global): %s", err.Error())
	}

	b.config = list.Config{}
	err = cfg.Section("bot").MapTo(&b.config)
	if err != nil {
		return fmt.Errorf("Config parse error (bot): %s", err.Error())
	}
	if debug {
		b.config.Debug = true
	}

	err = b.openDB()
	if err != nil {
		return fmt.Errorf("Database error: %s", err.Error())
	}

	return nil
}

func (b *SQLBackend) openDB() (err error) {
	b.db, err = sql.Open("sqlite3", b.Database)

	if err != nil {
		return
	}

	_, err = b.db.Exec(`
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

	return nil
}

func (b *SQLBackend) openLog() error {
	logFile, err := os.OpenFile(b.Log, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	out := io.MultiWriter(logFile, os.Stderr)
	log.SetOutput(out)
	return nil
}

func (b *SQLBackend) check(*kingpin.ParseContext) error {
	err := b.openLog()
	if err != nil {
		return fmt.Errorf("There's a problem with the log: %s", err.Error())
	}

	client, err := smtp.Dial(fmt.Sprintf("%s:%d", b.config.SMTPHostname, b.config.SMTPPort))
	if err != nil {
		return fmt.Errorf("There's a problem connecting to your SMTP server: %s", err.Error())
	}

	if b.config.SMTPUsername != "" {
		auth := smtp.PlainAuth("", b.config.SMTPUsername, b.config.SMTPPassword, b.config.SMTPHostname)
		err = client.Auth(auth)
		if err != nil {
			return fmt.Errorf("There's a problem authenticating with your SMTP server: %s", err.Error())
		}
	}

	return nil
}

func (b *SQLBackend) message(*kingpin.ParseContext) error {
	err := b.openLog()
	if err != nil {
		return err
	}

	bot := list.NewBot(b)
	return bot.Handle(bufio.NewReader(os.Stdin))
}

func (b *SQLBackend) Config() list.Config {
	return b.config
}

// Lists returns all lists
func (c *SQLBackend) Lists() ([]list.Definition, error) {
	rows, err := c.db.Query("SELECT list, name, description, hidden, locked, subscribers_only FROM lists ORDER BY list")
	if err != nil {
		return nil, err
	}

	result := []list.Definition{}
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

// LookupList returns a specific list, or nil if not found
func (b *SQLBackend) LookupList(name string) (*list.Definition, error) {
	rows, err := b.db.Query("SELECT list, name, description, hidden, locked, subscribers_only FROM lists WHERE list=?", name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	if rows.Next() {
		l, err := b.fetchList(rows)
		if err != nil {
			return nil, err
		}

		if rows.Next() {
			return nil, nil
		}

		return &l, nil
	}

	return nil, nil
}

func (b *SQLBackend) fetchList(rows *sql.Rows) (list.Definition, error) {
	l := list.Definition{}
	err := rows.Scan(&l.Address, &l.Name, &l.Description, &l.Hidden, &l.Locked, &l.SubscribersOnly)
	if err != nil {
		return l, err
	}
	l.Posters, err = b.listPosters(l.Address)
	if err != nil {
		return l, err
	}
	l.Bcc, err = b.listBcc(l.Address)
	return l, err
}

func (b *SQLBackend) listPosters(id string) ([]string, error) {
	rows, err := b.db.Query("SELECT address FROM posters WHERE list=?", id)
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

func (b *SQLBackend) listBcc(id string) ([]string, error) {
	rows, err := b.db.Query("SELECT address FROM bcc WHERE list=?", id)
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

// ListIsSubscribed method
func (b *SQLBackend) ListIsSubscribed(l list.Definition, user string) (*list.Subscription, error) {
	s := &list.Subscription{
		Address: user,
	}
	err := b.db.QueryRow("SELECT bounces, last_bounce FROM subscriptions WHERE user=? AND list=?", user, l.Address).Scan(&s.Bounces, &s.LastBounce)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return s, nil
}

// ListSubscribers method
func (b *SQLBackend) ListSubscribers(l list.Definition) ([]list.Subscription, error) {
	rows, err := b.db.Query("SELECT user, bounces, last_bounce FROM subscriptions WHERE list=?", l.Address)
	if err != nil {
		return nil, err
	}

	result := []list.Subscription{}
	defer rows.Close()
	for rows.Next() {
		s := list.Subscription{}
		err = rows.Scan(&s.Address, &s.Bounces, &s.LastBounce)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}

	return result, nil
}

// ListSubscribe method
func (b *SQLBackend) ListSubscribe(l list.Definition, user string) error {
	_, err := b.db.Exec("INSERT INTO subscriptions (user,list) VALUES(?,?)", user, l.Address)
	return err
}

// ListUnsubscribe method
func (b *SQLBackend) ListUnsubscribe(l list.Definition, user string) error {
	r, err := b.db.Exec("DELETE FROM subscriptions WHERE user=? AND list=?", user, l.Address)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("User %s is not subscribed to list %s", user, l.Address)
	}
	return nil
}

// ListSetBounce method
func (b *SQLBackend) ListSetBounce(l list.Definition, user string, bounces uint16, lastBounce time.Time) error {
	r, err := b.db.Exec("UPDATE subscriptions SET bounces = ?, last_bounce = ? WHERE user=? AND list=?", bounces, lastBounce, user, l.Address)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("User %s is not subscribed to list %s", user, l.Address)
	}
	return nil
}

// CreateList method
func (b *SQLBackend) CreateList(d list.Definition) error {
	tx, _ := b.db.Begin()

	_, err := tx.Exec("INSERT INTO lists (list, name, description, hidden, locked, subscribers_only) VALUES(?,?,?,?,?,?)",
		d.Address, d.Name, d.Description, d.Hidden, d.Locked, d.SubscribersOnly)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, address := range d.Posters {
		if address == "" {
			continue
		}

		_, err = tx.Exec("INSERT INTO posters (list, address) VALUES(?,?)", d.Address, address)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, address := range d.Bcc {
		if address == "" {
			continue
		}

		_, err = tx.Exec("INSERT INTO bcc (list, address) VALUES(?,?)", d.Address, address)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	tx.Commit()
	return nil
}

// ModifyList method
func (b *SQLBackend) ModifyList(a string, d list.Definition) error {
	tx, _ := b.db.Begin()

	r, err := tx.Exec("UPDATE lists SET list = ?, name = ?, description = ?, hidden = ?, locked = ?, subscribers_only = ? WHERE list = ?",
		d.Address, d.Name, d.Description, d.Hidden, d.Locked, d.SubscribersOnly, a)
	if err != nil {
		tx.Rollback()
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		tx.Rollback()
		return err
	}
	if n == 0 {
		tx.Rollback()
		return fmt.Errorf("List %s does not exist", a)
	}

	if a != d.Address {
		_, err := tx.Exec("UPDATE subscriptions SET list = ? WHERE list = ?", d.Address, a)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	_, err = tx.Exec("DELETE FROM posters WHERE list = ?", a)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, address := range d.Posters {
		_, err = tx.Exec("INSERT INTO posters (list, address) VALUES(?,?)", d.Address, address)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	_, err = tx.Exec("DELETE FROM bcc WHERE list = ?", a)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, address := range d.Bcc {
		_, err = tx.Exec("INSERT INTO bcc (list, address) VALUES(?,?)", d.Address, address)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	tx.Commit()
	return nil
}

// DeleteList method
func (b *SQLBackend) DeleteList(a string) error {
	tx, _ := b.db.Begin()

	_, err := tx.Exec("DELETE FROM subscriptions WHERE list = ?", a)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM bcc WHERE list = ?", a)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM posters WHERE list = ?", a)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM lists WHERE list = ?", a)
	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}
