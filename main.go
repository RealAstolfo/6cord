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
	"github.com/atotto/clipboard"
	"github.com/davecgh/go-spew/spew"
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
	rightflex     = tview.NewFlex()
	guildView     = tview.NewTreeView()
	messagesView  = tview.NewTextView()
	messagesFrame = tview.NewFrame(messagesView)
	wrapFrame     *tview.Frame
	input         = tview.NewInputField()
	autocomp      = tview.NewList()

	// ChannelID stores the current channel's ID
	ChannelID int64

	// LastAuthor stores for appending messages
	// TODO: migrate to table + lastRow
	LastAuthor int64

	d *discordgo.Session
)

func main() {
	guildView.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		// workaround to prevent crash when no root in tree
		return nil
	})

	messagesView.SetRegions(true)
	messagesView.SetWrap(true)
	messagesView.SetWordWrap(false)
	messagesView.SetScrollable(true)
	messagesView.SetDynamicColors(true)
	messagesView.SetBackgroundColor(BackgroundColor)

	token := flag.String("t", "", "Discord token (1)")

	username := flag.String("u", "", "Username/Email (2)")
	password := flag.String("p", "", "Password (2)")

	debug := flag.Bool("d", false, "Logs extra events")

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

	// Main app page

	appflex := tview.NewFlex()
	appflex.SetDirection(tview.FlexColumn)
	appflex.SetBackgroundColor(BackgroundColor)

	{ // Left container
		guildView.SetPrefixes([]string{"", "#"})
		guildView.SetTopLevel(1)
		guildView.SetBorder(true)
		guildView.SetTitle("Guilds")
		guildView.SetTitleAlign(tview.AlignLeft)
		guildView.SetBackgroundColor(BackgroundColor)

		appflex.AddItem(guildView, 0, 1, true)
	}

	{ // Right container
		rightflex.SetDirection(tview.FlexRow)
		rightflex.SetBackgroundColor(BackgroundColor)

		wrapFrame = tview.NewFrame(rightflex)
		wrapFrame.SetBorder(true)
		wrapFrame.SetBorders(0, 0, 0, 0, 0, 0)
		wrapFrame.SetTitle("")
		wrapFrame.SetTitleAlign(tview.AlignLeft)
		wrapFrame.SetTitleColor(tcell.ColorWhite)
		wrapFrame.SetBackgroundColor(BackgroundColor)

		autocomp.ShowSecondaryText(false)
		autocomp.SetBackgroundColor(BackgroundColor)

		autocomp.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			switch ev.Key() {
			case tcell.KeyDown:
				if autocomp.GetCurrentItem() == autocomp.GetItemCount()-1 {
					app.SetFocus(input)
					return nil
				}

				return ev

			case tcell.KeyUp:
				if autocomp.GetCurrentItem() < 1 {
					app.SetFocus(messagesView)
					return nil
				}

				return ev
			}

			return ev
		})

		input.SetPlaceholder("Send a message or input a command")
		input.SetFieldBackgroundColor(BackgroundColor)
		input.SetPlaceholderTextColor(tcell.ColorDarkCyan)

		input.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			switch ev.Key() {
			case tcell.KeyCtrlV:
				cb, err := clipboard.ReadAll()
				if err != nil {
					log.Println("Couldn't get clipboard:", err)
					return nil
				}

				input.SetText(input.GetText() + cb)

			case tcell.KeyLeft:
				if input.GetText() != "" {
					return ev
				}

				app.SetFocus(guildView)
				return nil

			case tcell.KeyUp:
				app.SetFocus(autocomp)

			case tcell.KeyEnter:
				if autocomp.GetItemCount() > 0 {
					autofillfunc(0)
					return nil
				}

				// log.Println(ev.Name())

				// if ev.Name() == "Shift+Enter" {
				// 	input.SetText(input.GetText() + "\\n")
				// 	return nil
				// }

				CommandHandler()

				return nil
			}

			return ev
		})

		input.SetChangedFunc(func(text string) {
			if len(text) == 0 {
				return
			}

			words := strings.Fields(text)

			if len(words) < 1 {
				return
			}

			switch last := words[len(words)-1]; {
			case strings.HasPrefix(last, "@"):
				fuzzyMentions(last)
			case strings.HasPrefix(last, ":"):
				fuzzyEmojis(last)
			}
		})

		messagesFrame.SetBorders(0, 0, 0, 0, 0, 0)
		messagesFrame.SetBackgroundColor(BackgroundColor)

		rightflex.AddItem(messagesFrame, 0, 1, false)
		rightflex.AddItem(autocomp, 1, 1, true)
		rightflex.AddItem(input, 1, 1, true)
		rightflex.SetBackgroundColor(BackgroundColor)

		appflex.AddItem(wrapFrame, 0, 3, true)
	}

	var showChannels = true

	messagesView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyPgDn, tcell.KeyPgUp, tcell.KeyUp, tcell.KeyDown:
			return event
		}

		app.SetFocus(input)
		return nil
	})

	autocomp.SetSelectedFunc(func(i int, a, b string, c rune) {
		autofillfunc(i)
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			app.Stop()

		case tcell.KeyF5:
			go func() {
				app.Stop()
				app.Run()
			}()

		case tcell.KeyTab:
			showChannels = !showChannels
			if showChannels {
				wrapFrame.SetBorder(true)
				appflex.RemoveItem(wrapFrame)

				appflex.AddItem(guildView, 0, 1, true)
				appflex.AddItem(wrapFrame, 0, 3, true)

				app.SetFocus(guildView)
			} else {
				wrapFrame.SetBorder(false)
				appflex.RemoveItem(guildView)

				app.SetFocus(input)
			}

			app.ForceDraw()
		}

		return event
	})

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

	discordgo.Logger = func(msgL, caller int, format string, a ...interface{}) {
		log.Println("Discordgo:", msgL, caller, format, a)

		if *debug {
			// Unsure if I should have spew as a dependency
			log.Println(spew.Sdump(a))
		}
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "rmkeyring":
			switch err := keyring.Delete(AppName, "token"); err {
			case nil:
				log.Println("Keyring deleted.")
				return
			default:
				log.Panicln(err)
			}
		}
	}

	d.AddHandler(onReady)
	d.AddHandler(messageCreate)
	d.AddHandler(messageUpdate)
	d.AddHandler(onTyping)
	d.AddHandler(messageAck)

	if *debug {
		d.AddHandler(func(s *discordgo.Session, r *discordgo.Resumed) {
			log.Println(spew.Sdump(r))
		})

		d.AddHandler(func(s *discordgo.Session, dc *discordgo.Disconnect) {
			log.Println(spew.Sdump(dc))
		})

		// d.AddHandler(func(s *discordgo.Session, i interface{}) {
		// 	log.Println(spew.Sdump(i))
		// })
	}

	// d.AddHandler(func(s *discordgo.Session, ev *discordgo.Event) {
	// 	log.Println(spew.Sdump(ev))
	// })

	d.StateEnabled = true

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
