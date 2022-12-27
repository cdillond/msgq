// package cms provides a bare-bones, unoptimized implementation of a content management system.
package cms

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrLoad   = errors.New("Article.PublishAt() and Article.DeleteAt() both returned false; article could not be loaded")
	ErrNotPub = errors.New("article was not published")
	ErrNotDel = errors.New("article was not deleted")
)

type Article interface {
	// Returns the time the Article should be published and a bool. If the bool is false (e.g. when deleting an already published article), the returned time.Time will be ignored and the Article will not be published.
	PublishAt() (time.Time, bool)

	// Returns the time the Article should be deleted and a bool. If the bool is false, the returned time.Time will be ignored and the Article will not be added to the deletion queue.
	DeleteAt() (time.Time, bool)

	// Returns the content of the Article, whatever form it might take. This will optionally be parsed by the Publisher or Deleter.
	Msg() any
}

type Publisher interface {
	Publish(msg any) error
}

type Deleter interface {
	Delete(msg any) error
}

// Listen takes a context.Context and a chan Article as an argument and sends Articles from an input source to the channel.
// *Important: Listen must also close ch and return when ctx is canceled.*
type Listener interface {
	Listen(ctx context.Context, ch chan Article)
}

type ErrorHandler interface {
	HandleError(err error, a Article)
}

type Cms struct {
	L           Listener
	P           Publisher
	D           Deleter
	E           ErrorHandler
	listenQueue chan Article
	pubQueue    chan Article
	delQueue    chan Article
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func sendToChan(a Article, ch chan Article) {
	ch <- a
}

func New(L Listener, P Publisher, D Deleter, E ErrorHandler) *Cms {
	ctx, cancel := context.WithCancel(context.Background())
	return &Cms{
		L: L,
		P: P,
		D: D,
		E: E,
		// these channels are buffered, but need not be
		listenQueue: make(chan Article, 100),
		pubQueue:    make(chan Article, 100),
		delQueue:    make(chan Article, 100),
		ctx:         ctx,
		cancel:      cancel,
		wg:          sync.WaitGroup{},
	}
}

// Run blocks and should always be invoked in a new goroutine.
func (c *Cms) Run() {
	delDone, pubDone := make(chan bool), make(chan bool)
	defer close(delDone)
	defer close(pubDone)
	go c.deleteQueue(delDone)
	go c.publishQueue(pubDone)
	go c.L.Listen(c.ctx, c.listenQueue)
	for {
		a, ok := <-c.listenQueue
		if !ok {
			// drain pubQueue and delQueue
			pubDone <- true
			delDone <- true
			go func() {
				for {
					a := <-c.pubQueue
					c.E.HandleError(ErrNotPub, a)
					c.wg.Done()
				}
			}()
			go func() {
				for {
					a := <-c.delQueue
					c.E.HandleError(ErrNotPub, a)
					c.wg.Done()
				}
			}()
			c.wg.Wait()
			close(c.pubQueue)
			close(c.delQueue)
			return
		}
		err := c.load(a)
		if err != nil {
			c.E.HandleError(err, a)
		}
	}
}

// load loads an article into either the publication queue or the deletion queue, but not both, and returns an error, which is nil if one of the operations is successful.
// If both a.PublishAt() and a.DeleteAt() return true, a is added to the deletion queue only after it has been published.
func (c *Cms) load(a Article) error {
	_, pub := a.PublishAt()
	if pub {
		go sendToChan(a, c.pubQueue)
		c.wg.Add(1)
		return nil
	}
	_, del := a.DeleteAt()
	if del {
		go sendToChan(a, c.delQueue)
		c.wg.Add(1)
		return nil
	}
	return ErrLoad
}

func (c *Cms) publishQueue(done chan bool) {
	for {
		select {
		case a := <-c.pubQueue:
			t, _ := a.PublishAt()
			if t.Before(time.Now()) {
				err := c.P.Publish(a.Msg())
				if err != nil {
					c.E.HandleError(err, a)
					c.wg.Done()
					continue
				}
				_, del := a.DeleteAt()
				if del {
					go sendToChan(a, c.delQueue)
				}
				c.wg.Done()
			} else {
				go sendToChan(a, c.pubQueue)
			}
		case <-done:
			return
		}
	}
}

func (c *Cms) deleteQueue(done chan bool) {
	for {
		select {
		case a := <-c.delQueue:
			t, _ := a.DeleteAt()
			if t.Before(time.Now()) {
				err := c.D.Delete(a.Msg())
				if err != nil {
					c.E.HandleError(err, a)
				}
				c.wg.Done()
			} else {
				go sendToChan(a, c.delQueue)
			}
		case <-done:
			return
		}
	}
}

func (c *Cms) ShutDown() {
	c.cancel()
}
