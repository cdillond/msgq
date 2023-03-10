// Package msgq provides a bare-bones, unoptimized implementation of a message queue.
package msgq

import (
	"errors"
	"time"
)

var (
	ErrLoad = errors.New("Message.ContentId() returned an empty string; message could not be loaded")
	ErrPub  = errors.New("MsgQ shut down before message was published")
)

type Message interface {
	// Returns the time at which the Message should be published.
	UpdateAt() time.Time
	// Returns the unique identifier for the content wrapped by the Message. This is used as they key when storing a Message in the MsgQ.Store queue.
	// If two Messages share a ContentId() return value, only one can be stored at a time; the first will be replaced by the second.
	ContentId() string
	// Publishes the content of the Message and returns a nil error on success.
	Publish() error
}

type ErrorHandler interface {
	HandleError(err error, msg Message)
}

type MsgQ struct {
	Store map[string]Message
}

func New() *MsgQ {
	return &MsgQ{
		Store: make(map[string]Message),
	}
}

// Run returns a channel that receives Messages and queues them for publication. Closing this channel shuts down the MsgQ. The chan struct{} that is also returned by Run() is notified when the shutdown is complete. Run() deletes Messages from the MsgQ.Store after Message.Publish() is called.
func (m *MsgQ) Run(E ErrorHandler) (chan Message, chan struct{}) {
	done1, done2 := make(chan struct{}), make(chan struct{})
	listenQueue := make(chan Message, 10)
	loadQueue := make(chan Message)
	go m.load(done1, loadQueue, E)
	go func() {
		for {
			msg, ok := <-listenQueue
			if !ok {
				// close loadQueue
				close(loadQueue)
				<-done1
				done2 <- struct{}{}
				return
			}
			id := msg.ContentId()
			if id == "" {
				E.HandleError(ErrLoad, msg)
			} else {
				loadQueue <- msg
			}
		}
	}()
	return listenQueue, done2
}

func (m *MsgQ) load(done chan struct{}, loadQueue chan Message, E ErrorHandler) {
	for {
		select {
		case msg, ok := <-loadQueue:
			if !ok {
				// handle unpublished Messages
				for _, msg := range m.Store {
					E.HandleError(ErrPub, msg)
				}
				done <- struct{}{}
				return
			} else {
				m.Store[msg.ContentId()] = msg
			}
		default:
			delList := make([]string, 0, len(m.Store))
			for id, msg := range m.Store {
				if msg.UpdateAt().Before(time.Now()) {
					// publish Message
					err := msg.Publish()
					if err != nil {
						E.HandleError(err, msg)
					}
					delList = append(delList, id)
				}
			}
			// remove published Messages from m.Store
			for i := 0; i < len(delList); i++ {
				delete(m.Store, delList[i])
			}
		}
	}
}
