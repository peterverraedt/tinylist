package list

import (
	"crypto/tls"
	"errors"
	"net/smtp"
)

// SendMail with InsecureSkipVerify set to true
func SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	c, err := smtp.Dial(addr)

	if err != nil {
		return err
	}

	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{InsecureSkipVerify: true}
		if err = c.StartTLS(config); err != nil {
			return err
		}
	}

	if a != nil {
		if ok, _ := c.Extension("AUTH"); !ok {
			return errors.New("smtp: server doesn't support AUTH")
		}

		if err = c.Auth(a); err != nil {
			return err
		}
	}

	if err = c.Mail(from); err != nil {
		return err
	}

	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := c.Data()

	if err != nil {
		return err
	}

	_, err = w.Write(msg)

	if err != nil {
		return err
	}

	err = w.Close()

	if err != nil {
		return err
	}

	return c.Quit()
}
