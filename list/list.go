package list

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// BounceInterval is used to compute ban times when bouncing
const BounceInterval = 7 * 24 * time.Hour

// A Definition defines a list definition
type Definition struct {
	Address         string   `ini:"address"`
	Name            string   `ini:"name"`
	Description     string   `ini:"description"`
	Hidden          bool     `ini:"hidden"`
	Locked          bool     `ini:"locked"`
	SubscribersOnly bool     `ini:"subscribers_only"`
	Posters         []string `ini:"posters,omitempty"`
	Bcc             []string `ini:"bcc,omitempty"`
}

func (def Definition) String() string {
	return fmt.Sprintf("%s <%s>: %s\nHidden: %v | Locked: %v | Subscribers only: %v\nPosters: %s\nBcc: %s",
		def.Name, def.Address, def.Description, def.Hidden, def.Locked, def.SubscribersOnly, strings.Join(def.Posters, ", "), strings.Join(def.Bcc, ", "))
}

// Subscription describes a subscription with metadata
type Subscription struct {
	Address    string
	Bounces    uint16
	LastBounce time.Time
}

// List represents a mailing list
type list struct {
	Definition
	Subscribe    func(string) error
	Unsubscribe  func(string) error
	SetBounce    func(string, uint16, time.Time) error
	Subscribers  func() ([]Subscription, error)
	IsSubscribed func(string) (*Subscription, error)
}

// CanPost checks if the user is authorised to post to this mailing list
func (list *list) CanPost(from string) bool {
	policy := true

	// Is there a whitelist of approved posters?
	if len(list.Posters) > 0 {
		policy = false

		for _, poster := range list.Posters {
			if from == poster {
				return true
			}
		}
	}

	// Is this list restricted to subscribers only?
	if list.SubscribersOnly {
		policy = false

		subscription, err := list.IsSubscribed(from)
		if err == nil && subscription != nil {
			return true
		}
	}

	return policy
}

// Send a message to the mailing list
func (list *list) Send(msg *Message, envelopeSender string, SMTPHostname string, SMTPPort uint64, SMTPUsername string, SMTPPassword string, debug bool) error {
	// Append list id to envelope sender
	parts := strings.SplitN(envelopeSender, "@", 2)
	if len(parts) < 2 {
		return fmt.Errorf("Invalid envelope sender %s", envelopeSender)
	}
	envelopeSender = fmt.Sprintf("%s+%s@%s", parts[0], strings.Replace(list.Address, "@", "=", 1), parts[1])

	// Collect recipients
	recipients := []string{}
	subscriptions, err := list.Subscribers()
	if err != nil {
		return err
	}
	for _, subscription := range subscriptions {
		ok, err := list.CheckBounces(subscription)
		if err != nil {
			return err
		}
		if ok {
			recipients = append(recipients, subscription.Address)
		}
	}
	for _, bcc := range list.Bcc {
		recipients = append(recipients, bcc)
	}

	// Send using VERP
	return msg.SendVERP(envelopeSender, recipients, SMTPHostname, SMTPPort, SMTPUsername, SMTPPassword, debug)
}

func (list *list) String() string {
	out := list.Definition.String() + "\nSubscribers:"
	subscribers, _ := list.Subscribers()
	for _, subscription := range subscribers {
		ok, _ := list.CheckBounces(subscription)
		if ok {
			out += fmt.Sprintf("\n  - %s (%d bounces, last on %s)", subscription.Address, subscription.Bounces, subscription.LastBounce)
		} else {
			out += fmt.Sprintf("\n  - %s (disabled, %d bounces, last on %s)", subscription.Address, subscription.Bounces, subscription.LastBounce)
		}
	}
	return out
}

// CheckBounces checks whether a user bounces too much. It returns true if the subscription should be considered active
func (list *list) CheckBounces(subscription Subscription) (bool, error) {
	if subscription.Bounces > 0 {
		var period time.Duration
		// First bounce is for free, after second bounce, wait 1 interval, after third bounce 2 intervals, then 4 intervals, 8 intervals...
		if subscription.Bounces > 1 {
			period = time.Duration(math.Pow(2, float64(subscription.Bounces-2))) * BounceInterval
		} else {
			period = 0
		}

		dontSendUntil := subscription.LastBounce.Add(period)
		now := time.Now()
		if now.Before(dontSendUntil) {
			return false, nil
		}
	}

	return true, nil
}
