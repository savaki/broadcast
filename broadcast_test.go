package broadcast_test

import (
	"bytes"
	"github.com/savaki/broadcast"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func TestCompiles(t *testing.T) {
	message1 := "hello world"
	message2 := "argle bargle"

	var publisher broadcast.Publisher
	var response chan *broadcast.Subscription
	var subscription *broadcast.Subscription
	var received string
	var err error

	Convey("Given a publisher", t, func() {
		publisher = broadcast.New()
		publisher.Start()

		Convey("When I subscribe to the publisher", func() {
			response = make(chan *broadcast.Subscription, 1)
			publisher.Subscribe(response)
			subscription = <-response

			Convey("Then I expect no errors", func() {
				So(subscription, ShouldNotBeNil)
			})

			Convey("Then I expect to receive published messages", func() {
				publisher.WriteString(message1)
				received = string(<-subscription.Receive)

				So(received, ShouldEqual, message1)

				Convey("When I unsubscribe", func() {
					publisher.Unsubscribe(subscription.Key)
					time.Sleep(10 * time.Millisecond) // delay is required to get the other go-routine to do work

					Convey("Then I expect to stop receiving messages", func() {
						publisher.WriteString(message1)
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
			if response != nil {
				close(response)
			}

			publisher.Close()
		})
	})

	Convey("When I attempt to start the publisher twice", t, func() {
		publisher = broadcast.New()

		// first start
		err = publisher.Start()
		So(err, ShouldBeNil)

		// second start
		err = publisher.Start()
		So(err, ShouldNotBeNil)

		publisher.Close()
	})
}

func TestMultiplePublishers(t *testing.T) {
	message := "hello world"

	var publisher1 broadcast.Publisher
	var publisher2 broadcast.Publisher
	var err error
	var buffer *bytes.Buffer

	Convey("Given two publishers", t, func() {
		buffer = &bytes.Buffer{}

		publisher1 = broadcast.New()
		err = publisher1.Start()
		So(err, ShouldBeNil)

		publisher2 = broadcast.New()
		err = publisher2.Start()
		So(err, ShouldBeNil)

		Convey("When I have publisher2 subscribe to publisher1", func() {
			go publisher1.SubscribeWriter(publisher2)
			go publisher2.SubscribeWriter(buffer)

			Convey("Then I expect data to be pushed to the downstream publisher", func() {
				time.Sleep(10 * time.Millisecond) // just enough time to swap go-routines
				publisher1.WriteString(message)

				time.Sleep(10 * time.Millisecond) // just enough time to swap go-routines
				So(buffer.String(), ShouldEqual, message)
			})
		})
	})
}
