broadcast
=========

[![GoDoc](https://godoc.org/github.com/savaki/broadcast?status.svg)](https://godoc.org/github.com/savaki/broadcast) [![Build Status](https://snap-ci.com/savaki/broadcast/branch/master/build_image)](https://snap-ci.com/savaki/broadcast/branch/master)

golang library to handling pub/sub to []byte streams

## Example

Very simple example of adding one subscriber to the publisher.

```
package main

import (
  "fmt"
  "github.com/savaki/broadcast"
)

func main() {
  messages := make(chan []byte, 512)

  // create a new publisher and attach it to this channel
  publisher := broadcast.New((<-chan []byte)(messages))
  publisher.Start() // VERY IMPORTANT - you must start the publisher
  defer publisher.Close()

  // subscribe to the publisher
  response := make(chan *broadcast.Subscription, 1)
  publisher.Subscribe(response)

  // wait to be subscribed
  subscription := <-response
  close(response)

  // send a message to the publisher
  messages <- []byte("hello world")
  received := <-subscription.Receive

  // should print out hello world
  fmt.Println("received message =>", string(received))
}
```


