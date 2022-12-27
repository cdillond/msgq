// Package cms provides a bare-bones, unoptimized implementation of a content management system.
package cms

import (
	"errors"
	"time"
)

var (
	ErrLoad = errors.New("message.Id() returned an empty string; message could not be loaded")
	ErrPub  = errors.New("message was not published")
)

type Message interface {
	// Returns the time at which the Message should be passed to the Publisher.
	UpdateAt() time.Time
	// Returns the unique identifier for the article wrapped by the Message. This is used as they key when storing a Message.
	// If two Messages share an article Id, only one can be stored at a time; the first will be replaced by the second.
	ArticleId() string
	// Returns the content of the Message, whatever form it might take.
	Article() any
}

type Publisher interface {
	Publish(msg Message)
}

type ErrorHandler interface {
	HandleError(err error, msg Message)
}

type Cms struct {
	Store map[string]Message
}

func New() *Cms {
	return &Cms{
		Store: make(map[string]Message),
	}
}

// Run returns a channel that receives Messages. Close the channel to shutdown the cms.
func (c *Cms) Run(P Publisher, E ErrorHandler) (chan Message, chan bool) {
	done1, done2 := make(chan bool), make(chan bool)
	listenQueue := make(chan Message, 100)
	loadQueue := make(chan Message)
	go c.load(done1, loadQueue, P, E)
	go func() {
		for {
			msg, ok := <-listenQueue
			if !ok {
				// close loadQueue
				close(loadQueue)
				<-done1
				done2 <- true
				return
			}
			id := msg.ArticleId()
			if id == "" {
				E.HandleError(ErrLoad, msg)
			} else {
				loadQueue <- msg
			}
		}
	}()
	return listenQueue, done2
}

func (c *Cms) load(done chan bool, loadQueue chan Message, P Publisher, E ErrorHandler) {
	for {
		select {
		case msg, ok := <-loadQueue:
			if !ok {
				// handle unpublished Messages
				for _, val := range c.Store {
					E.HandleError(ErrPub, val)
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
					// publish Message
					P.Publish(msg)
					delList = append(delList, msg.ArticleId())
				}
			}
			// remove published Messages from c.Store
			for i := 0; i < len(delList); i++ {
				delete(c.Store, delList[i])
			}
		}
	}
}
