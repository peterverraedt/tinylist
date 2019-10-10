package list

import (
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
// NotFound error is to be returned if no list or subscription is found
//NotFound = Error("Not found")
)

// A Backend can be used to create a bot
type Backend interface {
	Config() Config
	Lists() ([]Definition, error)
	CreateList(Definition) error
	ModifyList(string, Definition) error
	DeleteList(string) error
	LookupList(string) (*Definition, error)
	ListSubscribe(Definition, string) error
	ListUnsubscribe(Definition, string) error
	ListSetBounce(Definition, string, uint16, time.Time) error
	ListSubscribers(Definition) ([]Subscription, error)
	ListIsSubscribed(Definition, string) (*Subscription, error)
}

// A BotFactory creates a Bot based on the parsed context - before applying other actions
type botFactory func(*kingpin.ParseContext) *bot

// NewBotFactory creates a bot factory from a bot
func NewBotFactory(backend Backend) botFactory {
	return func(*kingpin.ParseContext) *bot {
		return NewBot(backend)
	}
}

// NewBot creates a new bot from a backend
func NewBot(backend Backend) *bot {
	b := &bot{}
	b.Config = backend.Config()

	b.Lists = func() ([]*list, error) {
		defs, err := backend.Lists()
		if err != nil {
			return nil, err
		}
		result := []*list{}
		for _, def := range defs {
			result = append(result, NewList(backend, def))
		}
		return result, nil
	}
	b.CreateList = func(def Definition) error {
		return backend.CreateList(def)
	}
	b.ModifyList = func(l *list, def Definition) error {
		return backend.ModifyList(l.Address, def)
	}
	b.DeleteList = func(l *list) error {
		return backend.DeleteList(l.Address)
	}
	b.LookupList = func(a string) (*list, error) {
		def, err := backend.LookupList(a)
		if err != nil {
			return nil, err
		}
		if def == nil {
			return nil, nil
		}
		return NewList(backend, *def), err
	}

	return b
}

// NewList creates a new list from a backend and a definition
func NewList(backend Backend, definition Definition) *list {
	l := &list{}
	l.Definition = definition

	l.Subscribe = func(a string) error {
		return backend.ListSubscribe(definition, a)
	}
	l.Unsubscribe = func(a string) error {
		return backend.ListUnsubscribe(definition, a)
	}
	l.SetBounce = func(a string, c uint16, t time.Time) error {
		return backend.ListSetBounce(definition, a, c, t)
	}
	l.Subscribers = func() ([]Subscription, error) {
		return backend.ListSubscribers(definition)
	}
	l.IsSubscribed = func(a string) (*Subscription, error) {
		return backend.ListIsSubscribed(definition, a)
	}

	return l
}
