package list

import (
	"fmt"
	"time"
	"strings"
)

// List represents a mailing list
type List struct {
	ID              string   `ini:"address"`
	Name            string   `ini:"name"`
	Description     string   `ini:"description"`
	Hidden          bool     `ini:"hidden"`
	Locked          bool     `ini:"locked"`
	SubscribersOnly bool     `ini:"subscribers_only"`
	Posters         []string `ini:"posters,omitempty"`
	Bcc             []string `ini:"bcc,omitempty"`
	Subscribe       func(string) error
	Unsubscribe     func(string) error
	SetBounce       func(string, uint16, time.Time) error
	Subscribers     func() ([]*Subscription, error)
	IsSubscribed    func(string) (*Subscription, error)
}

// Subscription describes a subscription with metadata
type Subscription struct {
	Address string
	Bounces uint16
	LastBounce time.Time
}

// CanPost checks if the user is authorised to post to this mailing list
func (list *List) CanPost(from string) bool {

	// Is this list restricted to subscribers only?
	if list.SubscribersOnly {
		subscription, err := list.IsSubscribed(from)
		if err != nil || subscription == nil {
			return false
		}
	}

	// Is there a whitelist of approved posters?
	if len(list.Posters) > 0 {
		for _, poster := range list.Posters {
			if from == poster {
				return true
			}
		}
		return false
	}

	return true
}

// Send a message to the mailing list
func (list *List) Send(msg *Message, envelopeSender string, SMTPHostname string, SMTPPort uint64, SMTPUsername string, SMTPPassword string, debug bool) error {
	// Append list id to envelope sender
	parts := strings.SplitN(envelopeSender, "@", 2)
	if len(parts) < 2 {
		return fmt.Errorf("Invalid envelope sender %s", envelopeSender)
	}
	envelopeSender = fmt.Sprintf("%s+%s@%s", parts[0], strings.ReplaceAll(list.ID, "@", "="), parts[1])
	
	// Collect recipients
	recipients := []string{}
	subscriptions, err := list.Subscribers()
	if err != nil {
		return err
	}
	for _, subscription := range subscriptions {
		recipients = append(recipients, subscription.Address)
	}
	for _, bcc := range list.Bcc {
		recipients = append(recipients, bcc)
	}

	// Send using VERP
	errors := msg.SendVERP(envelopeSender, recipients, SMTPHostname, SMTPPort, SMTPUsername, SMTPPassword, debug)
	if len(errors) > 0 {
		return fmt.Errorf("%d errors occurred during sending: %v", len(errors), errors)
	}
	return nil
}

func (list *List) String() string {
	subscribers, _ := list.Subscribers()
	out := fmt.Sprintf("Name: %s <%s>\nDescription: %s\nHidden: %v | Locked: %v | Subscribers only: %v\nPosters: %v\nBcc: %v\nSubscribers:",
	  list.Name, list.ID, list.Description, list.Hidden, list.Locked, list.SubscribersOnly, list.Posters, list.Bcc)
	for _, subscription := range subscribers {
		out += fmt.Sprintf("\n  - %s (%d bounces, last on %s)", subscription.Address, subscription.Bounces, subscription.LastBounce)
	}
	return out
}
