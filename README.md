nanolist
========

nanolist is a lightweight mailing list manager written in Go. It's easy to
deploy, and easy to manage. It was written as an antithesis of the experience
of setting up other mailing list software.

Usage
-----

nanolist is controlled by emailing nanolist with a command in the subject.

The following commands are available:

* `help` - Reply with a list of valid commands
* `lists` - Reply with a list of available mailing lists
* `subscribe list-id` - Subscribe to receive mail sent to the given list
* `unsubscribe list-id` - Unsubscribe from receiving mail sent to the given list

Frequently Asked Questions
--------------------------

### Is there a web interface?

No. If you'd like an online browsable archive of emails, I recommend looking
into tools such as hypermail, which generate HTML archives from a list of
emails.

If you'd like to advertise the lists on your website, it's recommended to do
that manually, in whatever way looks best. Subscribe buttons can be achieved
with a `mailto:` link.

### How do I integrate this with my preferred mail transfer agent?

I'm only familiar with postfix, for which there are instructions below. The
gist of it is: have your mail server pipe emails for any mailing list addresses
to `nanolist message`. nanolist will handle any messages sent to it this way,
and reply using the configured SMTP server.

### Why would anyone want this?

Some people prefer mailing lists for patch submission and review, some people
want to play mailing-list based games such as nomic, and some people are just
nostalgic.

Installation
------------

First, you'll need to build and install the nanolist binary:
`go get github.com/eXeC64/nanolist`

Second, you'll need to write a config to either `/etc/nanolist.ini`
or `/usr/local/etc/nanolist.ini` as follows:

You can also specify a custom config file location by invoking nanolist
with the `-config` flag: `-config=/path/to/config.ini`

```ini
# File for event and error logging. nanolist does not rotate its logs
# automatically. Recommended path is /var/log/mail/nanolist
# You'll need to set permissions on it depending on which account your MTA
# runs nanolist as.
log = /path/to/logfile

# An sqlite3 database is used for storing the email addresses subscribed to
# each mailing list. Recommended location is /var/db/nanolist.db
# You'll need to set permissions on it depending on which account your MTA
# runs nanolist as.
database = /path/to/sqlite/database

[bot]
# Address nanolist should receive user commands on
command_address = lists@example.com

# SMTP details for sending mail
smtp_hostname = "smtp.example.com"
smtp_port = 25
smtp_username = "nanolist"
smtp_password = "hunter2"
```

Create a list by invoking
```bash
nanolist create --list=golang@example.com --name="Go programming" --description="General discussion of Go programming" --bcc archive@example.com --bcc datahoarder@example.com
nanolist create --list=announce@example.com --name="Announcements" --description="Important announcements" --poster admin@example.com --poster moderator@example.com
nanolist create --list=robertpaulson99@example.com --name "fight club" --flag subscribers_only --flag hidden
```

Lastly, you need to hook the desired incoming addresses to nanolist:

In `/etc/aliases`:
```
nanolist: "| /path/to/bin/nanolist message"
```

And run `newaliases` for the change to take effect.

This creates an alias that pipes messages sent to the `nanolist` alias to the
nanolist command.

The final step is telling your preferred MTA to route mail to this address
when needed.

For postfix edit `/etc/postfix/aliases` and add:
```
lists@example.com nanolist
golang@example.com nanolist
announce@example.com nanolist
robertpaulson99@example.com nanolist
```
and restart postfix.

Congratulations, you've now set up 3 mailing lists of your own!

License
-------

nanolist is made available under the BSD-3-Clause license.
