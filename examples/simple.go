package main

import (
	"fmt"
	"github.com/savaki/broadcast"
)

func main() {
	// create a new publisher and attach it to this channel
	publisher := broadcast.New()
	publisher.Start() // VERY IMPORTANT - you must start the publisher
	defer publisher.Close()

	// subscribe to the publisher
	response := make(chan *broadcast.Subscription, 1)
	publisher.Subscribe(response)

	// wait to be subscribed
	subscription := <-response
	close(response)

	// send a message to the publisher
	publisher.WriteString("hello world")
	received := <-subscription.Receive

	// should print out hello world
	fmt.Println("received message =>", string(received))
}
