package httpMultiplexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type jsonurl struct {
	URLS []string
}

type Multi struct {
	parent     *httpMultiplexer
	httpclient *http.Client
	maxurls    uint
	TimeOut    time.Duration
}

func (mult *Multi) urlsHandler(rw http.ResponseWriter, r *http.Request, jurls *jsonurl, ctx context.Context, handlereturn chan string) {
	if len(jurls.URLS) > int(mult.maxurls) {
		fmt.Println(mult.maxurls)
		mult.printResponeError(rw, errors.New(fmt.Sprintf("Max urls in this server:%v", mult.maxurls)))
		return
	}
	for i := 0; i < len(jurls.URLS); i++ {
		fmt.Println(jurls.URLS[i])
	}
	ch := make(chan bool)
	go func() {
		time.Sleep(13 * time.Second)
		ch <- true
	}()
	select {
	case <-ctx.Done():
		return
	case <-ch:
		fmt.Println("yay")
		rw.Write([]byte("ss"))
	}
}

func (mult *Multi) HandleHttp(rw http.ResponseWriter, r *http.Request, ctx context.Context, handlereturn chan string) {
	defer r.Body.Close()
	readbyte, err := io.ReadAll(r.Body)
	if err != nil {
		mult.printResponeError(rw, err)
		return
	}
	var urls jsonurl
	if err := json.Unmarshal(readbyte, &urls); err != nil {
		mult.printResponeError(rw, err)
		return
	}
	mult.urlsHandler(rw, r, &urls, ctx, handlereturn)
	defer mult.parent.DonePost()
}

func (mult *Multi) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		mult.printResponeError(rw, errors.New("The server only accepts POST requests."))
		return
	}
	if err := mult.parent.AddPost(); err != nil {
		mult.printResponeError(rw, err)
		return
	}
	handlereturn := make(chan string)
	ctx, cancelCtx := context.WithCancel(r.Context())
	go mult.HandleHttp(rw, r, ctx, handlereturn)
	timeout := make(chan bool)
	go func() {
		time.Sleep(mult.TimeOut)
		timeout <- true
	}()
	select {
	case <-r.Context().Done():
		cancelCtx()
		return
	case <-timeout:
		cancelCtx()
		mult.printResponeError(rw, errors.New("Server Timeout"))
		return
	case out, closed := <-handlereturn:
		if !closed {
			return
		}
		rw.Write([]byte(out))
	}
}

func (mult *Multi) printResponeError(rw http.ResponseWriter, err error) {
	rw.Write([]byte("Error:" + err.Error()))
	mult.parent.DonePost()
}

type httpMultiplexer struct {
	httpserv     *http.Server
	MaxPostCount uint
	MaxGetCount  uint
	thisPost     uint
	multiplex    *Multi
}

func (hsr *httpMultiplexer) DonePost() {
	hsr.thisPost = hsr.thisPost - 1
}

func (hsr *httpMultiplexer) AddPost() error {
	if hsr.thisPost+1 > hsr.MaxPostCount {
		return errors.New("Http Post Limit")
	}
	hsr.thisPost = hsr.thisPost + 1
	return nil
}

func (hsr *httpMultiplexer) Init(maxPostCount uint, maxGetCount uint, maxurls uint) {
	hsr.MaxPostCount = maxPostCount
	hsr.MaxGetCount = maxGetCount
	hsr.multiplex.maxurls = maxurls
	hsr.httpserv.Handler = hsr.multiplex
	hsr.httpserv.ListenAndServe()
}

func NewhttpMultiplexer(port string, serverTimeout uint, clientTimeout uint) *httpMultiplexer {
	NewMultiplexer := &httpMultiplexer{
		httpserv: &http.Server{Addr: "localhost:" + port},
		multiplex: &Multi{
			httpclient: &http.Client{
				Timeout: time.Duration(clientTimeout) * time.Second},
			TimeOut: time.Duration(serverTimeout) * time.Second}}
	NewMultiplexer.multiplex.parent = NewMultiplexer
	return NewMultiplexer
}
