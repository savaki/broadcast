package broadcast_test

import (
	"github.com/savaki/broadcast"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestCompiles(t *testing.T) {
	message1 := "hello world"
	message2 := "argle bargle"

	var messages chan []byte
	var publisher broadcast.Publisher
	var response chan *broadcast.Subscription
	var subscription *broadcast.Subscription
	var received string
	var err error

	Convey("Given a publisher", t, func() {
		messages = make(chan []byte, 16)
		publisher = broadcast.New((<-chan []byte)(messages))
		publisher.Start()

		Convey("When I subscribe to the publisher", func() {
			response = make(chan *broadcast.Subscription, 1)
			publisher.Subscribe(response)
			subscription = <-response

			Convey("Then I expect no errors", func() {
				So(subscription, ShouldNotBeNil)
			})

			Convey("Then I expect to receive published messages", func() {
				messages <- []byte(message1)
				received = string(<-subscription.Receive)

				So(received, ShouldEqual, message1)

				Convey("When I unsubscribe", func() {
					publisher.Unsubscribe(subscription.Key)
					time.Sleep(10 * time.Millisecond) // delay is required to get the other go-routine to do work

					Convey("Then I expect to stop receiving messages", func() {
						messages <- []byte(message1)
						select {
						case data := <-subscription.Receive:
							received = string(data)
						case <-time.After(10 * time.Millisecond):
						}

						So(received, ShouldNotEqual, message2)
					})
				})

				Convey("When I then close the publisher", func() {
					publisher.Close()

					Convey("Then I expect the receive channel to be closed", func() {
						_, open := <-subscription.Receive

						So(open, ShouldBeFalse)
					})
				})
			})
		})

		Reset(func() {
			if messages != nil {
				close(messages)
			}
			if response != nil {
				close(response)
			}

			publisher.Close()
		})
	})

	Convey("When I attempt to start the publisher twice", t, func() {
		messages = make(chan []byte, 16)
		publisher = broadcast.New((<-chan []byte)(messages))

		// first start
		err = publisher.Start()
		So(err, ShouldBeNil)

		// second start
		err = publisher.Start()
		So(err, ShouldNotBeNil)

		publisher.Close()
	})
}
