package main

type List struct {
	Name            string `ini:"name"`
	Description     string `ini:"description"`
	Id              string
	Address         string   `ini:"address"`
	Hidden          bool     `ini:"hidden"`
	Locked          bool     `ini:"locked"`
	SubscribersOnly bool     `ini:"subscribers_only"`
	Posters         []string `ini:"posters,omitempty"`
	Bcc             []string `ini:"bcc,omitempty"`
}

// Check if the user is authorised to post to this mailing list
func (list *List) CanPost(from string) bool {

	// Is this list restricted to subscribers only?
	if list.SubscribersOnly && !isSubscribed(from, list.Id) {
		return false
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
func (list *List) Send(msg *Message) {
	recipients := fetchSubscribers(list.Id)
	for _, bcc := range list.Bcc {
		recipients = append(recipients, bcc)
	}
	msg.Send(recipients)
}
