package main

import (
	"flag"
	"log"
	"os"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/RumbleFrog/discordgo"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	keyring "github.com/zalando/go-keyring"
)

const (
	// AppName used for keyrings
	AppName = "6cord"
)

var (
	app           = tview.NewApplication()
	guildView     = tview.NewTreeView()
	messagesView  = tview.NewTextView()
	messagesFrame = tview.NewFrame(messagesView)
	wrapFrame     *tview.Frame
	input         = tview.NewInputField()

	// ChannelID stores the current channel's ID
	ChannelID int64

	// LastAuthor stores for appending messages
	// TODO: migrate to table + lastRow
	LastAuthor int64

	d *discordgo.Session
)

func main() {
	sender := strings.NewReplacer(
		`\n`, "\n",
		`\t`, "\t",
	)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			app.Stop()
		}

		return event
	})

	guildView.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		// workaround to prevent crash when no root in tree
		return nil
	})

	messagesView.SetRegions(true)
	messagesView.SetWrap(true)
	messagesView.SetWordWrap(true)
	messagesView.SetScrollable(true)
	messagesView.SetDynamicColors(true)

	token := flag.String("t", "", "Discord token (1)")

	username := flag.String("u", "", "Username/Email (2)")
	password := flag.String("p", "", "Password (2)")

	flag.Parse()

	var login []interface{}

	k, err := keyring.Get(AppName, "token")
	if err != nil {
		if err != keyring.ErrNotFound {
			log.Println(err.Error())
		}

		switch {
		case *token != "":
			login = append(login, *token)
		case *username != "", *password != "":
			login = append(login, *username)
			login = append(login, *password)

			if *token != "" {
				login = append(login, *token)
			}
		default:
			log.Fatalln("Token OR username + password missing! Refer to -h")
		}
	} else {
		login = append(login, k)
	}

	d, err = discordgo.New(login...)
	if err != nil {
		log.Panicln(err)
	}

	d.State.MaxMessageCount = 50

	appflex := tview.NewFlex().SetDirection(tview.FlexColumn)

	{ // Left container
		guildView.SetPrefixes([]string{"", "", "#"})
		guildView.SetTopLevel(1)
		appflex.AddItem(guildView, 0, 1, true)
	}

	{ // Right container
		flex := tview.NewFlex().SetDirection(tview.FlexRow)
		flex.SetBackgroundColor(tcell.ColorDefault)

		wrapFrame = tview.NewFrame(flex)
		wrapFrame.SetBorder(true)
		wrapFrame.SetTitle("")
		wrapFrame.SetTitleAlign(tview.AlignLeft)
		wrapFrame.SetTitleColor(tcell.ColorWhite)

		input.SetBackgroundColor(tcell.ColorAqua)
		input.SetPlaceholder("Send a message or input a command")

		input.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			switch ev.Key() {
			case tcell.KeyLeft:
				if input.GetText() != "" {
					return ev
				}

				app.SetFocus(guildView)
				return nil
			case tcell.KeyEnter:
				text := input.GetText()
				if text == "" {
					return nil
				}

				text = sender.Replace(text)

				go func(text string) {
					if _, err := d.ChannelMessageSend(ChannelID, text); err != nil {
						log.Println(err)
					}
				}(text)

				input.SetText("")

				return nil
			}

			return ev
		})

		input.SetChangedFunc(func(text string) {})

		messagesFrame.SetBorders(0, 0, 0, 0, 0, 0)

		flex.AddItem(messagesFrame, 0, 1, false)
		flex.AddItem(input, 1, 1, true)

		appflex.AddItem(wrapFrame, 0, 3, true)
	}

	app.SetRoot(appflex, true)

	logFile, err := os.OpenFile(
		"/tmp/6cord.log",
		os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC,
		0664,
	)

	if err != nil {
		panic(err)
	}

	defer logFile.Close()

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// if len(os.Args) > 1 {
	// 	switch os.Args[1] {
	// 	case "rmkeyring":
	// 		switch err := keyring.Delete(AppName, "token"); err {
	// 		case nil:
	// 			log.Println("Keyring deleted.")
	// 			return
	// 		default:
	// 			log.Panicln(err)
	// 		}
	// 	}
	// }

	d.AddHandler(onReady)
	d.AddHandler(messageCreate)
	d.AddHandler(messageUpdate)
	d.AddHandler(onTyping)

	// d.AddHandler(func(s *discordgo.Session, ev *discordgo.Event) {
	// 	log.Println(spew.Sdump(ev))
	// })

	// d.AddHandler(func(s *discordgo.Session, a *discordgo.MessageAck) {
	// 	log.Println(spew.Sdump(a))
	// })

	if err := d.Open(); err != nil {
		log.Fatalln("Failed to connect to Discord", err.Error())
	}

	defer d.Close()
	defer app.Stop()

	log.Println("Storing token inside keyring...")
	if err := keyring.Set(AppName, "token", d.Token); err != nil {
		log.Println("Failed to set keyring! Continuing anyway...", err.Error())
	}

	if err := app.Run(); err != nil {
		log.Panicln(err)
	}
}
