package broadcast

import (
	"code.google.com/p/go-uuid/uuid"
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

type state uint64

const (
	initialized state = iota
	running
	closed
)

type Key string

type Publisher interface {
	// start the publisher
	Start() error

	// create a new subscription; once a subscription is created, the information will be passed back on the provided
	// channel
	Subscribe(chan *Subscription) error

	// have messages automatically published to the specified writer.  this is a BLOCKING call
	SubscribeWriter(io.Writer)

	// unsubscribe the specified key from the Publisher; if this key doesn't exist, nothing will happen
	Unsubscribe(Key)

	// close the publisher.  this will close all the underlying channels
	Close()

	// broadcast data through this publisher
	Write([]byte) (int, error)

	// convenience methods around #Write
	WriteString(string) (int, error)
}

type Subscription struct {
	Key     Key
	Receive <-chan []byte
}

type subscriber struct {
	Send chan []byte
}

func New() Publisher {
	return &publisher{
		subscribers: make(map[Key]*subscriber),
		messages:    make(chan []byte, 4096),
		subscribe:   make(chan chan *Subscription, 32),
		unsubscribe: make(chan Key, 32),
		done:        make(chan interface{}),
		wg:          &sync.WaitGroup{},
	}
}

type publisher struct {
	subscribers map[Key]*subscriber
	messages    chan []byte
	subscribe   chan chan *Subscription
	unsubscribe chan Key
	done        chan interface{}
	wg          *sync.WaitGroup
	state       state
}

func newKey() Key {
	return Key(uuid.NewRandom().String())
}

func (p *publisher) agent() {
	p.wg.Add(1)
	defer p.wg.Done()

	for {
		select {
		case response := <-p.subscribe:
			key := newKey()
			value := &subscriber{
				Send: make(chan []byte, 1024),
			}
			p.subscribers[key] = value
			s := &Subscription{
				Key:     key,
				Receive: value.Send,
			}
			response <- s

		case key := <-p.unsubscribe:
			if subscriber, ok := p.subscribers[key]; ok {
				delete(p.subscribers, key)
				close(subscriber.Send)
			}

		case message, open := <-p.messages:
			if message == nil && !open {
				continue
			}
			for key, subscriber := range p.subscribers {
				select {
				case subscriber.Send <- message:
				default:
					delete(p.subscribers, key)
					close(subscriber.Send)
				}
			}

		case <-p.done:
			// close all the subscribers
			for key, subscriber := range p.subscribers {
				delete(p.subscribers, key)
				close(subscriber.Send)
			}

			close(p.messages)

			// drain any subscription requests so they won't wait for us
			for {
				select {
				case response := <-p.subscribe:
					response <- &Subscription{Key: newKey()}
				default:
					return
				}
			}
		}
	}
}

func (p *publisher) transitionState(from, to state) bool {
	return atomic.CompareAndSwapUint64((*uint64)(&p.state), uint64(from), uint64(to))
}

func (p *publisher) Start() error {
	if ok := p.transitionState(initialized, running); !ok {
		return errors.New("Publisher was already started")
	}

	go p.agent()
	return nil
}

func (p *publisher) Subscribe(response chan *Subscription) error {
	if p.state != running {
		return errors.New("unable to subscribe.  is this publisher started?")
	}

	p.subscribe <- response
	return nil
}

// have messages automatically published to the specified writer.  this is a BLOCKING call
func (p *publisher) SubscribeWriter(target io.Writer) {
	response := make(chan *Subscription)
	p.Subscribe(response)
	subscription := <-response
	defer close(response)
	defer p.Unsubscribe(subscription.Key)

	t := target
	for data := range subscription.Receive {
		_, err := t.Write(data)
		if err != nil {
			return
		}
	}
}

func (p *publisher) Unsubscribe(key Key) {
	if p.state == running {
		p.unsubscribe <- key
	}
}

func (p *publisher) Close() {
	// indicate this publisher is closed
	if ok := p.transitionState(running, closed); ok {
		p.done <- true
		p.wg.Wait()
	}
}

func (p *publisher) Write(data []byte) (int, error) {
	if data == nil {
		return 0, errors.New("illegal attempt to write nil to the Publisher")
	} else if p.state == running {
		p.messages <- data
		return len(data), nil
	} else {
		return 0, errors.New("unable to write to a publisher that is not running.  perhaps you didn't call #Start?")
	}
}

func (p *publisher) WriteString(content string) (int, error) {
	return p.Write([]byte(content))
}
