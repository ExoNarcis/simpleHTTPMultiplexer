package httpMultiplexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
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

func (mult *Multi) httpGet(ctx context.Context, url string) (*http.Response, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	var bytesresp []byte
	bytesresp, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	return resp, string(bytesresp), nil
}

func (mult *Multi) httpGetCl(urls []string, ctx context.Context, myrequest chan string, errorchan chan error) {
	defer close(myrequest)
	defer func() {
		recover()
	}()
	for i := 0; i < len(urls); i++ {
		ctxd, _ := context.WithTimeout(ctx, mult.ClientTimeOut)
		resp, body, err := mult.httpGet(ctxd, urls[i])
		if err != nil {
			errorchan <- err
			break
		}
		jsong := &jsonGet{Url: urls[i], Header: resp.Header.Get("User-Agent"), Body: body}
		var out []byte
		out, err = json.Marshal(jsong)
		if err != nil {
			errorchan <- err
			break
		}
		select {
		case <-ctx.Done():
			break
		default:
			myrequest <- string(out)
		}
	}
}

func (mult *Multi) getforchannels(ctx context.Context, resultchan chan string, RequestChannes []chan string, urlcount int) {
	fullstring := "["
	var count int
	if urlcount%int(mult.MaxGetCount) != 0 {
		count = (urlcount / int(mult.MaxGetCount)) + 1
	} else {
		count = urlcount / int(mult.MaxGetCount)
	}
	for i := 0; i < count; i++ {
		for _, v := range RequestChannes {
			select {
			case <-ctx.Done():
				return
			case value := <-v:
				if fullstring != "[" {
					fullstring = fullstring + "," + value
				} else {
					fullstring = fullstring + value
				}
			}

		}
	}
	select {
	case <-ctx.Done():
		return
	default:
		resultchan <- fullstring + "]"
	}
}

func (mult *Multi) genGeneralChannels(urls []string, ctx context.Context, resultchan chan string, errorchan chan error) {
	var RequestChannes []chan string
	urlslen := len(urls)
	getter := make(chan string)
	if urlslen > int(mult.MaxGetCount) {
		for i := 0; i < int(mult.MaxGetCount); i++ {
			RequestChannes = append(RequestChannes, make(chan string))
		}
		k := int(mult.MaxGetCount)
		chanint := 0
		go mult.getforchannels(ctx, getter, RequestChannes, urlslen)
		for i := 0; i < urlslen; k, i, chanint = k-1, i+(urlslen-i)/k, chanint+1 {
			go mult.httpGetCl(urls[i:i+(urlslen-i)/k], ctx, RequestChannes[chanint], errorchan)
		}
	} else {
		for i := 0; i < urlslen; i++ {
			RequestChannes = append(RequestChannes, make(chan string))
		}
		go mult.getforchannels(ctx, getter, RequestChannes, urlslen)
		for i := 0; i < urlslen; i++ {
			go mult.httpGetCl(urls[i:i+1], ctx, RequestChannes[i], errorchan)
		}
	}
	select {
	case <-ctx.Done():
		return
	case fullstr := <-getter:
		resultchan <- fullstr
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
	ctxe, cancelF := context.WithCancel(ctx)
	go mult.genGeneralChannels(jurls.URLS, ctxe, result, errequesr)
	select {
	case <-ctx.Done():
		return
	case err := <-errequesr:
		cancelF()
		mult.printResponeError(rw, err)
		close(handlereturn)
		return
	case requerstreturn := <-result:
		handlereturn <- requerstreturn
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
	ctx, cancelCtx := context.WithTimeout(r.Context(), mult.TimeOut)
	go mult.HandleHttp(rw, r, ctx, handlereturn)
	select {
	case <-r.Context().Done():
		cancelCtx()
		return
	case <-ctx.Done():
		mult.printResponeError(rw, errors.New("Server Timeout"))
		return
	case out, notclosed := <-handlereturn:
		if notclosed {
			rw.Write([]byte(out))
			savechanclose(handlereturn)
		}
	}
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

func (hsr *httpMultiplexer) Init(maxPostCount uint, maxGetCount uint, maxurls uint, saveshutdown bool) {
	hsr.multiplex.MaxPostCount = maxPostCount
	hsr.multiplex.MaxGetCount = maxGetCount
	hsr.multiplex.maxurls = maxurls
	hsr.httpserv.Handler = hsr.multiplex
	var save sync.WaitGroup
	save.Add(1)
	if saveshutdown {
		go func() {
			osSig := make(chan os.Signal)
			signal.Notify(osSig, os.Interrupt)
			<-osSig
			if err := hsr.httpserv.Shutdown(context.Background()); err != nil {
				fmt.Println("HTTP SERVER SHUTDOWN ERR:" + err.Error())
			}
			save.Done()
		}()
	}
	fmt.Println("STARTING HTTP SERVER")
	if err := hsr.httpserv.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Println("HTTP SERVER ERR:" + err.Error())
		save.Done()
	}
	save.Wait()
}

func NewhttpMultiplexer(port string, serverTimeout uint, clientTimeout uint) *httpMultiplexer {
	NewMultiplexer := &httpMultiplexer{
		httpserv: &http.Server{Addr: ":" + port},
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
