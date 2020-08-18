package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"

	"github.com/chzyer/readline"
	"github.com/gookit/color"
	"github.com/kc2g-flex-tools/flexclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var dst string

func init() {
	flag.StringVar(&dst, "radio", ":discover:", "Radio to connect to")
}

func main() {
	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{
			Out: os.Stderr,
		},
	).With().Timestamp().Logger()
	flag.Parse()

	fc, err := flexclient.NewFlexClient(dst)
	if err != nil {
		log.Fatal().Err(err).Msg("creating client")
	}

	prompt := color.FgLightMagenta.Render("flex") + "> "
	rl, err := readline.New(prompt)
	if err != nil {
		log.Fatal().Err(err).Msg("creating readline")
	}
	log.Logger = zerolog.New(
		zerolog.ConsoleWriter{
			Out: rl.Stderr(),
		},
	).With().Timestamp().Logger()

	// Display messages
	go func() {
		messages := make(chan flexclient.Message)
		fc.SetMessageChan(messages)
		for msg := range messages {
			fmt.Fprintln(
				rl.Stdout(),
				color.FgLightGreen.Render("MSG"), " ",
				msg.Message,
			)
		}
	}()

	// Display updates
	go func() {
		updates := make(chan flexclient.StateUpdate, 10)
		sub := fc.Subscribe(flexclient.Subscription{"", updates})
		for upd := range updates {
			out := color.FgLightGreen.Render("UPD") + " "
			out += color.FgLightBlue.Render(upd.SenderHandle) + " "
			out += color.FgLightYellow.Render(upd.Object) + ": "

			keys := []string{}
			for key := range upd.CurrentState {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			space := ""
			for _, key := range keys {
				var keyColor, valueColor color.Color
				if _, ok := upd.Updated[key]; ok {
					keyColor = color.LightCyan
					valueColor = color.LightWhite
				} else {
					keyColor = color.Cyan
					valueColor = color.White
				}

				out += space + keyColor.Render(key) + "=" + valueColor.Render(upd.CurrentState[key])
				space = " "
			}
			fmt.Fprintln(rl.Stdout(), out)
		}
		fc.Unsubscribe(sub)
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		fc.Run()
		wg.Done()
	}()

	fc.StartUDP()
	fc.SendAndWait("sub slice all")

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		fmt.Println("Exit on SIGINT")
		fc.Close()
	}()

	wg.Add(1)

	go func() {
		for {
			line, err := rl.Readline()
			if err != nil {
				break
			}
			if line == "" {
				continue
			}

			res := fc.SendAndWait(line)
			statusColor := color.FgLightGreen
			if res.Error > 0 {
				statusColor = color.FgLightRed
			}
			fmt.Fprintln(
				rl.Stdout(),
				color.FgLightGreen.Render("RES"), " ",
				color.FgLightBlue.Render(res.Serial), " ",
				statusColor.Render(fmt.Sprintf("%08X", res.Error)),
			)
		}
		fc.Close()
		wg.Done()
	}()

	wg.Wait()
}
