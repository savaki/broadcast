package broadcast

import (
	"code.google.com/p/go-uuid/uuid"
	"errors"
	"sync"
	"sync/atomic"
)

type State uint64

const (
	Initialized State = iota
	Running
	Closed
)

type Key string

type Publisher interface {
	// start the publisher
	Start() error

	// create a new subscription; once a subscription is created, the information will be passed back on the provided
	// channel
	Subscribe(chan *Subscription)

	// unsubscribe the specified key from the Publisher; if this key doesn't exist, nothing will happen
	Unsubscribe(Key)

	// close the publisher.  this will close all the underlying channels
	Close()
}

type Subscription struct {
	Key     Key
	Receive <-chan []byte
}

type subscriber struct {
	Send chan []byte
}

func New(messages <-chan []byte) Publisher {
	return &publisher{
		subscribers: make(map[Key]*subscriber),
		messages:    messages,
		subscribe:   make(chan chan *Subscription, 32),
		unsubscribe: make(chan Key, 32),
		done:        make(chan interface{}),
		wg:          &sync.WaitGroup{},
	}
}

type publisher struct {
	subscribers map[Key]*subscriber
	messages    <-chan []byte
	subscribe   chan chan *Subscription
	unsubscribe chan Key
	done        chan interface{}
	wg          *sync.WaitGroup
	state       State
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

func (p *publisher) transitionState(from, to State) bool {
	return atomic.CompareAndSwapUint64((*uint64)(&p.state), uint64(from), uint64(to))
}

func (p *publisher) Start() error {
	if ok := p.transitionState(Initialized, Running); !ok {
		return errors.New("Publisher was already started")
	}

	go p.agent()
	return nil
}

func (p *publisher) Subscribe(response chan *Subscription) {
	if p.state == Running {
		p.subscribe <- response
	}
}

func (p *publisher) Unsubscribe(key Key) {
	if p.state == Running {
		p.unsubscribe <- key
	}
}

func (p *publisher) Close() {
	// indicate this publisher is closed
	if ok := p.transitionState(Running, Closed); ok {
		p.done <- true
		p.wg.Wait()
	}
}
