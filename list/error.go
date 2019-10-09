package list

// A BotError is a structured error caused by the bot
type BotError struct {
	Message string
}

func (e *BotError) Error() string {
	return e.Message
}
