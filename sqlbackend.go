package main

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/smtp"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/peterverraedt/tinylist/list"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/ini.v1"
)

// An SQLBackend is an implementation of list.Backend
type SQLBackend struct {
	Log      string `ini:"log"`
	Driver   string `ini:"driver"`
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
		cfg, err = ini.LooseLoad("tinylist.ini", "/usr/local/etc/tinylist.ini", "/etc/tinylist.ini")
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
	var (
		driver  string
		queries []string
	)

	switch b.Driver {
	case "mysql":
		driver = "mysql"

		queries = append(queries,
			`CREATE TABLE IF NOT EXISTS lists (
				list VARCHAR(255) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				description VARCHAR(255) NOT NULL,
				hidden INTEGER(1) NOT NULL,
				locked INTEGER(1) NOT NULL,
				subscribers_only INTEGER(1) NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS bcc (
				list VARCHAR(255) NOT NULL,
				address VARCHAR(255) NOT NULL,
				UNIQUE KEY list_address (list,address)
			)`,
			`CREATE TABLE IF NOT EXISTS posters (
				list VARCHAR(255) NOT NULL,
				address VARCHAR(255) NOT NULL,
				UNIQUE KEY list_address (list,address)
			)`,
			`CREATE TABLE IF NOT EXISTS subscriptions (
				list VARCHAR(255) NOT NULL,
				user VARCHAR(255) NOT NULL,
				bounces INTEGER NOT NULL DEFAULT 0,
				last_bounce DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00',
				UNIQUE KEY list_user (list,user)
			)`,
			`CREATE TABLE IF NOT EXISTS archive (
				list VARCHAR(255) NOT NULL,
				id VARCHAR(255) NOT NULL,
				sender VARCHAR(255) NOT NULL,
				subject VARCHAR(255) NOT NULL,
				date DATETIME NOT NULL,
				message LONGBLOB NOT NULL,
				UNIQUE KEY list_id (list,id)
			)`)
	default:
		driver = "sqlite3"

		queries = append(queries,
			`CREATE TABLE IF NOT EXISTS lists (
				list TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				description TEXT NOT NULL,
				hidden INTEGER(1) NOT NULL,
				locked INTEGER(1) NOT NULL,
				subscribers_only INTEGER(1) NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS bcc (
				list TEXT NOT NULL,
				address TEXT NOT NULL,
				UNIQUE(list,address)
			)`,
			`CREATE TABLE IF NOT EXISTS posters (
				list TEXT NOT NULL,
				address TEXT NOT NULL,
				UNIQUE(list,address)
			)`,
			`CREATE TABLE IF NOT EXISTS subscriptions (
				list TEXT NOT NULL,
				user TEXT NOT NULL,
				bounces INTEGER NOT NULL DEFAULT 0,
				last_bounce DATETIME NOT NULL DEFAULT 0,
				UNIQUE(list,user)
			)`,
			`CREATE TABLE IF NOT EXISTS archive (
				list TEXT NOT NULL,
				id TEXT NOT NULL,
				sender TEXT NOT NULL,
				subject TEXT NOT NULL,
				date DATETIME NOT NULL,
				message BLOB NOT NULL,
				UNIQUE(list,id)
			)`)
	}

	b.db, err = sql.Open(driver, b.Database)
	if err != nil {
		return
	}

	for _, query := range queries {
		_, err = b.db.Exec(query)
		if err != nil {
			return
		}
	}

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
		l, err := c.fetchList(rows.Scan)
		if err != nil {
			return nil, err
		}

		result = append(result, l)
	}

	return result, rows.Err()
}

// LookupList returns a specific list, or nil if not found
func (b *SQLBackend) LookupList(name string) (*list.Definition, error) {
	row := b.db.QueryRow("SELECT list, name, description, hidden, locked, subscribers_only FROM lists WHERE list=?", name)

	if err := row.Err(); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	l, err := b.fetchList(row.Scan)
	if err != nil {
		return nil, err
	}

	return &l, nil
}

func (b *SQLBackend) fetchList(scan func(dest ...interface{}) error) (list.Definition, error) {
	l := list.Definition{}

	err := scan(&l.Address, &l.Name, &l.Description, &l.Hidden, &l.Locked, &l.SubscribersOnly)
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

	return result, rows.Err()
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

	return result, rows.Err()
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

	return result, rows.Err()
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
		return fmt.Errorf("user %s is not subscribed to list %s", user, l.Address)
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
		return fmt.Errorf("user %s is not subscribed to list %s", user, l.Address)
	}

	return nil
}

// ListArchive method.
func (b *SQLBackend) ListArchive(l list.Definition, msg *list.Message) error {
	var (
		data = []byte(msg.String())
		id   = fmt.Sprintf("%x", sha256.Sum256(data))
		now  = time.Now()
	)

	_, err := b.db.Exec(`INSERT INTO archive (list,id,sender,subject,date,message) VALUES(?,?,?,?,?,?)`,
		l.Address,
		id,
		msg.From,
		msg.Subject,
		now,
		data)

	return err
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
