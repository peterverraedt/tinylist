package list

import (
	"fmt"
)

// List represents a mailing list
type List struct {
	ID              string
	Name            string   `ini:"name"`
	Description     string   `ini:"description"`
	Address         string   `ini:"address"`
	Hidden          bool     `ini:"hidden"`
	Locked          bool     `ini:"locked"`
	SubscribersOnly bool     `ini:"subscribers_only"`
	Posters         []string `ini:"posters,omitempty"`
	Bcc             []string `ini:"bcc,omitempty"`
	Subscribe       func(string) error
	Unsubscribe     func(string) error
	Subscribers     func() ([]string, error)
	IsSubscribed    func(string) (bool, error)
}

// CanPost checks if the user is authorised to post to this mailing list
func (list *List) CanPost(from string) bool {

	// Is this list restricted to subscribers only?
	if list.SubscribersOnly {
		ok, err := list.IsSubscribed(from)
		if err != nil || !ok {
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
	recipients, err := list.Subscribers()
	if err != nil {
		return err
	}
	for _, bcc := range list.Bcc {
		recipients = append(recipients, bcc)
	}
	return msg.Send(envelopeSender, recipients, SMTPHostname, SMTPPort, SMTPUsername, SMTPPassword, debug)
}

func (list *List) String() string {
	subscribers, _ := list.Subscribers()
	return fmt.Sprintf("List %s (%s):\n  Name: %s\n  Description: %s\n  Hidden: %v | Locked: %v | Subscribers only: %v\n  Posters: %v\n  Bcc: %v\n  Subscribers: %v",
		list.ID, list.Address, list.Name, list.Description, list.Hidden, list.Locked, list.SubscribersOnly, list.Posters, list.Bcc, subscribers)
}