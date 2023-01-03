# msgq
I created this to play around with channels in Go. It should not be considered production ready. This package is intended to be as general and adaptable as possible, so it provides very little in the way of specific features.
\
Example:
```go
// implement the Message interface
type DefaultMessage struct {
    Id string
    Content string
    Time time.Time
}
func (d DefaultMessage) UpdateAt() time.Time {
    return d.Time
}
func (d DefaultMessage) ContentId() string {
    return d.Id
}
// realistically, the Publish() method will be much more complex
func (d DefaultMessage) Publish() error {
    fmt.Println(d.Content)
    return nil
}

// implement the ErrorHandler interface
type DefaultErrorHandler struct{}
func (d DefaultErrorHandler) HandleError(err error, msg msgq.Message) {
    fmt.Println(err.Error(), msg)
}

func main() {
    MsgQ := msgq.New()
    listenChan, done := MsgQ.Run()
    
    //TODO more complex example
    source := []DefaultMessage{{"1","content", time.Now().Add(time.Second)}, {"2", "content 2", time.Now()}}

    for _, msg := range source {
        listenChan <- msg
    }
    // wait for messages to be published
    time.Sleep(3*time.Second)
    // shut down
    close(listenChan)
    <- done
}
```
