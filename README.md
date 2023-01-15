# msgq
<a href="https://pkg.go.dev/github.com/cdillond/msgq"><img src="https://pkg.go.dev/badge/github.com/cdillond/msgq.svg" alt="Go Reference"></a>\
I created this to play around with channels in Go. It should not be considered production ready. This package is intended to be as general and adaptable as possible, so it provides very little in the way of specific features.

## Example
```go
// implement the Message interface
type ExampleMessage struct {
    Id string
    Content string
    Time time.Time
}
func (e ExampleMessage) UpdateAt() time.Time {
    return e.Time
}
func (e ExampleMessage) ContentId() string {
    return e.Id
}
// realistically, the Publish() method will contain more complicated logic
func (e ExampleMessage) Publish() error {
    fmt.Println(e.Content)
    return nil
}

// implement the ErrorHandler interface
type ExampleErrorHandler struct{}
func (e ExampleErrorHandler) HandleError(err error, msg msgq.Message) {
    fmt.Println(err.Error(), msg)
}

func main() {
    MsgQ := msgq.New()
    listenChan, done := MsgQ.Run()
    
    //TODO more complex example
    source := []ExampleMessage{{"1","content", time.Now().Add(time.Second)}, {"2", "content 2", time.Now()}}

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
