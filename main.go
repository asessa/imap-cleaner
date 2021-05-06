package main

import (
	"fmt"
	"os"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func inSlice(value string, slice []string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func Cleanup(host string, port int, user, pass string, mailboxes []string, from, to *time.Time, list, restore, expunge, verbose bool) error {
	if verbose {
		fmt.Printf("connecting to %s:%d ...", host, port)
	}

	// Connect to server
	c, err := client.DialTLS(fmt.Sprintf("%s:%d", host, port), nil)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Println("done")
	}

	defer c.Logout()

	// Login
	if verbose {
		fmt.Printf("login as %s ...", user)
	}
	if err := c.Login(user, pass); err != nil {
		return err
	}
	if verbose {
		fmt.Println("done")
	}

	// Retreive a list of available mailboxes
	availableMailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", availableMailboxes)
	}()
	if err := <-done; err != nil {
		return err
	}

	// loop through all mailboxes
	for m := range availableMailboxes {
		if list {
			fmt.Printf("%s\n", m.Name)
			continue
		}

		// skip if a mailbox has been passed and the current mailbox is not in that list.
		if len(mailboxes) > 0 && !inSlice(m.Name, mailboxes) {
			continue
		}

		// Select INBOX
		if verbose {
			fmt.Printf("processing %s\n", m.Name)
		}
		if _, err := c.Select(m.Name, false); err != nil {
			return err
		}

		// check if we have a delete filter by date range.
		if from != nil || to != nil {
			search := imap.NewSearchCriteria()

			dbg := "deleting messages:"

			if from != nil {
				search.Since = *from
				dbg += " from " + from.Format("2006-01-02 15:04:05")
			}

			if to != nil {
				search.Before = *to
				dbg += " to " + to.Format("2006-01-02 15:04:05")
			}

			if verbose {
				fmt.Printf("%s\n", dbg)
			}

			ids, err := c.Search(search)
			if err != nil {
				return err
			}

			if len(ids) > 0 {
				seqset := new(imap.SeqSet)
				seqset.AddNum(ids...)

				messages := make(chan *imap.Message, len(ids))
				done := make(chan error, 1)
				go func() {
					done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
				}()
				if err := <-done; err != nil {
					return err
				}

				updates := make(chan *imap.Message, 1)
				var item imap.StoreItem
				if restore {
					item = imap.FormatFlagsOp(imap.RemoveFlags, true)
				} else {
					item = imap.FormatFlagsOp(imap.AddFlags, true)
				}
				if err := c.Store(seqset, item, []interface{}{imap.DeletedFlag}, updates); err != nil {
					return err
				}
			}
		}

		// Expunge
		if expunge {
			if verbose {
				fmt.Println("expunging deleted messages")
			}
			if err := c.Expunge(nil); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkPassword(c *cli.Context) (string, error) {
	// check if password is passsed
	pass := c.String("pass")
	if len(pass) == 0 {
		// reader := bufio.NewReader(os.Stdin)
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			term.Restore(int(os.Stdin.Fd()), oldState)
			return "", err
		}

		t := term.NewTerminal(os.Stdin, "")
		pass, err = t.ReadPassword("Enter Password: ")
		term.Restore(int(os.Stdin.Fd()), oldState)
		if err != nil {
			return "", err
		}
	}

	return pass, nil
}

func main() {
	var port int

	globalFlags := []cli.Flag{
		&cli.BoolFlag{
			Name:        "verbose",
			Aliases:     []string{"v"},
			Usage:       "Verbose mode",
			DefaultText: "false",
		},
		&cli.StringFlag{
			Name:     "host",
			Usage:    "IMAP server host or ip",
			Required: true,
		},
		&cli.IntFlag{
			Name:        "port",
			Usage:       "IMAP server port",
			DefaultText: "993",
			Required:    false,
		},
		&cli.StringFlag{
			Name:     "user",
			Aliases:  []string{"u"},
			Usage:    "IMAP username",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "pass",
			Aliases:  []string{"p"},
			Usage:    "IMAP password, if password is not given it's asked from the tty.",
			Required: false,
		},
		&cli.StringSliceFlag{
			Name:  "mailbox",
			Usage: "specify a mailbox (defaults to all account mailboxes)",
		},
	}

	flagsDelete := []cli.Flag{
		&cli.TimestampFlag{
			Name:        "from",
			Usage:       "delete messages starting from specified date",
			Layout:      "2006-01-02 15:04:05",
			DefaultText: "nil",
		},
		&cli.TimestampFlag{
			Name:        "to",
			Usage:       "delete messages up to specified date",
			Layout:      "2006-01-02 15:04:05",
			DefaultText: "nil",
		},
	}

	err := (&cli.App{
		Version: "0.1.0",
		Name:    "imap-cleaner",
		Usage:   "cleanup and expunge IMAP folders",
		Commands: []*cli.Command{
			{
				Name:        "cleanup",
				Description: "delete and expunge messages",
				Flags:       append(globalFlags, flagsDelete...),
				Action: func(c *cli.Context) error {
					pass, err := checkPassword(c)
					if err != nil {
						return err
					}

					return Cleanup(c.String("host"), port, c.String("user"), pass, c.StringSlice("mailbox"), c.Timestamp("from"), c.Timestamp("to"), false, false, true, c.Bool("verbose"))
				},
			},
			{
				Name:        "delete",
				Description: "delete messages",
				Flags:       append(globalFlags, flagsDelete...),
				Action: func(c *cli.Context) error {
					pass, err := checkPassword(c)
					if err != nil {
						return err
					}

					return Cleanup(c.String("host"), port, c.String("user"), pass, c.StringSlice("mailbox"), c.Timestamp("from"), c.Timestamp("to"), false, false, false, c.Bool("verbose"))
				},
			},
			{
				Name:        "restore",
				Description: "restore deleted messages (not yet expunged)",
				Flags:       append(globalFlags, flagsDelete...),
				Action: func(c *cli.Context) error {
					pass, err := checkPassword(c)
					if err != nil {
						return err
					}

					return Cleanup(c.String("host"), port, c.String("user"), pass, c.StringSlice("mailbox"), c.Timestamp("from"), c.Timestamp("to"), false, true, false, c.Bool("verbose"))
				},
			},
			{
				Name:        "expunge",
				Description: "expunge deleted messages",
				Flags:       globalFlags,
				Action: func(c *cli.Context) error {
					pass, err := checkPassword(c)
					if err != nil {
						return err
					}

					return Cleanup(c.String("host"), port, c.String("user"), pass, c.StringSlice("mailbox"), nil, nil, false, false, true, c.Bool("verbose"))
				},
			},
			{
				Name:        "list",
				Description: "list available mailboxes",
				Flags:       globalFlags,
				Action: func(c *cli.Context) error {
					pass, err := checkPassword(c)
					if err != nil {
						return err
					}

					return Cleanup(c.String("host"), port, c.String("user"), pass, []string{}, nil, nil, true, false, false, c.Bool("verbose"))
				},
			},
		},
		Before: func(c *cli.Context) error {
			// check port
			port = c.Int("port")
			if port == 0 {
				port = 993
			}

			return nil
		},
	}).Run(os.Args)
	if err != nil {
		fmt.Println()
		fmt.Println(err)
	}
}
