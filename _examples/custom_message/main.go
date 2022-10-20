package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/myzhan/boomer"
)

// This is an example about how to subscribe to locust's custom events.

var globalBoomer = boomer.NewBoomer("127.0.0.1", 5557)

func foo() {
	time.Sleep(3 * time.Second)
}

func eventHandler(msg *boomer.CustomMessage) {
	log.Printf("Custom message recv: %v\n", msg)

	// Avoid doing lots of type casting between python and go, we stringify data before sending to boomer.
	data, ok := msg.Data.([]byte)
	if !ok {
		log.Println("Failed to cast msg.data to []byte")
	} else {
		log.Printf("data: %s", string(data))
	}
	globalBoomer.SendCustomMessage("acknowledge_users", "Thanks for the message")
}

func waitForQuit() {
	wg := sync.WaitGroup{}
	wg.Add(1)

	quitByMe := false
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		quitByMe = true
		globalBoomer.Quit()
		wg.Done()
	}()

	boomer.Events.Subscribe(boomer.EVENT_QUIT, func() {
		if !quitByMe {
			wg.Done()
		}
	})

	wg.Wait()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	boomer.Events.Subscribe("test_users", eventHandler)

	task := &boomer.Task{
		Name:   "foo",
		Weight: 10,
		Fn:     foo,
	}

	globalBoomer.Run(task)

	waitForQuit()
	log.Println("shutdown")
}
