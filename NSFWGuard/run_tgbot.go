package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"NSFWGuard/tgbot"
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGKILL)

	botCtrl, err := tgbot.NewBotCtrl()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("HERE WE GO!")
	select {
	case s := <-sigs:
		log.Printf("Got signal %v", s)
		botCtrl.Stop()
	}
	log.Printf("I'll be back...")
}
