// package cms provides a bare-bones, unoptimized implementation of a content management system.
package cms

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrLoad = errors.New("message.Id() returned an empty string; message could not be loaded")
	ErrPub  = errors.New("message was not published")
)

type Message interface {
	// Returns the time at which the Message should be passed to the Publisher.
	UpdateAt() time.Time
	// Optionally returns user-defined methods for the Message (e.g. "DELETE", "PUBLISH", etc).
	// This is not used by the cms package directly, but may be helfpul in an implementation of a Publisher.
	Method() string
	// Returns the unique identifier for the article wrapped by the Message. This is used as they key when storing a Message.
	// If two Messages share an article Id, only one can be stored at a time; the first will be replaced by the second.
	ArticleId() string
	// Returns the content of the Message, whatever form it might take.
	Article() any
	// Optionally returns an identifier for the Message. This is not used directly by the cms package, but may be useful for error handling.
	MessageId() string
}

type Publisher interface {
	Publish(msg Message)
}

type Listener interface {
	// Listen takes a context.Context and a chan Message as an argument and sends Messages from an input source to the channel.
	// *Important: Listen must also close ch and return when ctx is canceled.*
	Listen(ctx context.Context, ch chan Message)
}

type ErrorHandler interface {
	HandleError(err error, msg Message)
}

type Cms struct {
	L           Listener
	P           Publisher
	E           ErrorHandler
	Store       map[string]Message
	listenQueue chan Message
	loadQueue   chan Message
	done        chan bool
	ctx         context.Context
	cancel      context.CancelFunc
}

func New(L Listener, P Publisher, E ErrorHandler) *Cms {
	ctx, cancel := context.WithCancel(context.Background())
	return &Cms{
		L:           L,
		P:           P,
		E:           E,
		Store:       make(map[string]Message),
		listenQueue: make(chan Message, 100),
		loadQueue:   make(chan Message),
		done:        make(chan bool),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Run blocks and should always be invoked in a new goroutine.
func (c *Cms) Run() {
	go c.L.Listen(c.ctx, c.listenQueue)
	go c.load(c.done)
	for {
		msg, ok := <-c.listenQueue
		if !ok {
			// close loadQueue
			close(c.loadQueue)
			return
		}
		id := msg.ArticleId()
		if id == "" {
			c.E.HandleError(ErrLoad, msg)
		} else {
			c.loadQueue <- msg
		}
	}
}

func (c *Cms) load(done chan bool) {
	for {
		select {
		case msg, ok := <-c.loadQueue:
			if !ok {
				// de-queue
				fmt.Println("there should be some output here...")
				fmt.Println(c.Store)
				for _, val := range c.Store {
					fmt.Println("leftover")
					c.E.HandleError(ErrPub, val)
				}
				done <- true
				return
			} else {
				c.Store[msg.ArticleId()] = msg
			}
		default:
			delList := make([]string, 0, len(c.Store))
			for _, msg := range c.Store {
				if msg.UpdateAt().Before(time.Now()) {
					// PUBLISH UPDATE
					c.P.Publish(msg)
					delList = append(delList, msg.ArticleId())
				}
			}
			for i := 0; i < len(delList); i++ {
				delete(c.Store, delList[i])
			}
		}
	}
}

func (c *Cms) ShutDown() {
	c.cancel()
	<-c.done
}
