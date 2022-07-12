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

type jsonGet struct {
	Url    string
	Header string
	Body   string
}

type jsonurl struct {
	URLS []string
}

type Multi struct {
	MaxPostCount  uint
	MaxGetCount   uint
	thisPost      uint
	ClientTimeOut time.Duration
	TimeOut       time.Duration
	parent        *httpMultiplexer
	maxurls       uint
}

func (mult *Multi) genGeneralChannels(urls []string, ctx context.Context, resultchan chan string, errorchan chan error) {
	var RequestChannes []chan string
	urlslen := len(urls)
	if urlslen >= int(mult.MaxGetCount) {
		RequestChannes = make([]chan string, mult.MaxGetCount)
		defer savechancloseAll(RequestChannes)
		for i := 0; i < int(mult.MaxGetCount); i++ {
			RequestChannes = append(RequestChannes, make(chan string))
		}
		for i := 0; i < urlslen; i += int(mult.MaxGetCount) {
			fmt.Println(urls[i : i+int(mult.MaxGetCount)])
		}
	} else {
		RequestChannes = make([]chan string, urlslen)
		defer savechancloseAll(RequestChannes)
		for i := 0; i < urlslen; i++ {
			RequestChannes = append(RequestChannes, make(chan string))
		}
		for i := 0; i < urlslen; i++ {
			fmt.Println(urls[i])
		}
	}
}

func (mult *Multi) urlsHandler(rw http.ResponseWriter, r *http.Request, jurls *jsonurl, ctx context.Context, handlereturn chan string) {
	if len(jurls.URLS) > int(mult.maxurls) {
		mult.printResponeError(rw, errors.New(fmt.Sprintf("Max urls in this server:%v", mult.maxurls)))
		close(handlereturn)
		return
	}
	errequesr := make(chan error)
	defer savechanclose(errequesr)
	result := make(chan string)
	defer savechanclose(result)
	go mult.genGeneralChannels(jurls.URLS, ctx, result, errequesr)
	select {
	case <-ctx.Done():
		return
	case err := <-errequesr:
		mult.printResponeError(rw, err)
		close(handlereturn)
	case requerstreturn := <-result:
		handlereturn <- requerstreturn
		return
	}
}

func (mult *Multi) HandleHttp(rw http.ResponseWriter, r *http.Request, ctx context.Context, handlereturn chan string) {
	readbyte, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		mult.printResponeError(rw, err)
		close(handlereturn)
		return
	}
	var urls jsonurl
	if err := json.Unmarshal(readbyte, &urls); err != nil {
		mult.printResponeError(rw, err)
		close(handlereturn)
		return
	}
	mult.urlsHandler(rw, r, &urls, ctx, handlereturn)
}

func (mult *Multi) checkreques(r *http.Request) error {
	if r.Method != "POST" {
		return errors.New("The server only accepts POST requests.")
	}
	if err := mult.AddPost(); err != nil {
		return err
	}
	return nil
}

func (mult *Multi) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	defer mult.DonePost()
	if err := mult.checkreques(r); err != nil {
		mult.printResponeError(rw, err)
		return
	}
	handlereturn := make(chan string)
	defer savechanclose(handlereturn)
	ctx, cancelCtx := context.WithTimeout(r.Context(), mult.TimeOut)
	go mult.HandleHttp(rw, r, ctx, handlereturn)
	select {
	case <-r.Context().Done():
		cancelCtx()
	case <-ctx.Done():
		mult.printResponeError(rw, errors.New("Server Timeout"))
	case out, notclosed := <-handlereturn:
		if notclosed {
			rw.Write([]byte(out))
		}
	}
	return
}

func (mult *Multi) printResponeError(rw http.ResponseWriter, err error) {
	rw.Write([]byte("Error:" + err.Error()))
}

func (mult *Multi) DonePost() {
	if mult.thisPost > 0 {
		mult.thisPost = mult.thisPost - 1
	}
}

func (mult *Multi) AddPost() error {
	if mult.thisPost+1 > mult.MaxPostCount {
		return errors.New("Http Post Limit")
	}
	mult.thisPost = mult.thisPost + 1
	return nil
}

type httpMultiplexer struct {
	httpserv  *http.Server
	multiplex *Multi
}

func (hsr *httpMultiplexer) Init(maxPostCount uint, maxGetCount uint, maxurls uint) {
	hsr.multiplex.MaxPostCount = maxPostCount
	hsr.multiplex.MaxGetCount = maxGetCount
	hsr.multiplex.maxurls = maxurls
	hsr.httpserv.Handler = hsr.multiplex
	hsr.httpserv.ListenAndServe()
}

func NewhttpMultiplexer(port string, serverTimeout uint, clientTimeout uint) *httpMultiplexer {
	NewMultiplexer := &httpMultiplexer{
		httpserv: &http.Server{Addr: "localhost:" + port},
		multiplex: &Multi{
			ClientTimeOut: time.Duration(clientTimeout) * time.Second,
			TimeOut:       time.Duration(serverTimeout) * time.Second}}
	NewMultiplexer.multiplex.parent = NewMultiplexer
	return NewMultiplexer
}

func savechanclose[T any](channel chan T) {
	defer func() {
		recover()
	}()
	select {
	case _, notclosed := <-channel:
		if notclosed {
			close(channel)
		}
	default:
		close(channel)
	}
}
func savechancloseAll[T any](channel []chan T) {
	for i := 0; i < len(channel); i++ {
		savechanclose(channel[i])
	}
}
