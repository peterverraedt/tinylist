package list

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// List represents a mailing list
type List struct {
	Address         string   `ini:"address"`
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
	Address    string
	Bounces    uint16
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

func (list *List) String() string {
	subscribers, _ := list.Subscribers()
	out := fmt.Sprintf("Name: %s <%s>\nDescription: %s\nHidden: %v | Locked: %v | Subscribers only: %v\nPosters: %v\nBcc: %v\nSubscribers:",
		list.Name, list.Address, list.Description, list.Hidden, list.Locked, list.SubscribersOnly, list.Posters, list.Bcc)
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
func (list *List) CheckBounces(subscription *Subscription) (bool, error) {
	if subscription.Bounces > 0 {
		var period time.Duration
		// First bounce is for free, after second bounce, wait 1 day, after third bounce 2 days, then 4 days, 8 days...
		if subscription.Bounces > 1 {
			period = time.Duration(math.Pow(2, float64(subscription.Bounces-2))) * 24 * time.Hour
		} else {
			period = 0
		}

		dontSendUntil := subscription.LastBounce.Add(period)
		now := time.Now()
		if now.Before(dontSendUntil) {
			return false, nil
		}

		// Forget about bounces if long ago
		clearBounces := dontSendUntil.Add(period).Add(24 * time.Hour)
		if now.Before(clearBounces) {
			err := list.SetBounce(subscription.Address, 0, now)
			return true, err
		}
	}

	return true, nil
}
