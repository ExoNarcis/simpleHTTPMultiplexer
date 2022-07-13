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

func (mult *Multi) httpGetCl(urls []string, ctx context.Context, myrequest chan string, errorchan chan error) {
	for i := 0; i < len(urls); i++ {
		select {
		case <-ctx.Done():
			return
		default:
			ctxd, _ := context.WithTimeout(ctx, mult.ClientTimeOut)
			req, err := http.NewRequestWithContext(ctxd, http.MethodGet, urls[i], nil)
			if err != nil {
				errorchan <- err
				return
			}
			var resp *http.Response
			resp, err = http.DefaultClient.Do(req)
			if err != nil {
				errorchan <- err
				return
			}
			var bytesresp []byte
			bytesresp, err = io.ReadAll(resp.Body)
			if err != nil {
				errorchan <- err
				return
			}
			defer resp.Body.Close()
			jsong := &jsonGet{Url: urls[i], Header: resp.Header.Get("User-Agent"), Body: string(bytesresp)}
			var out []byte
			out, err = json.Marshal(jsong)
			if err != nil {
				errorchan <- err
				return
			}
			myrequest <- string(out)
		}
	}
	close(myrequest)
}

func (mult *Multi) getforchannels(resultchan chan string, RequestChannes []chan string, urlcount int) {
	var fullstring string
	var count int
	if urlcount%int(mult.MaxGetCount) != 0 {
		count = (urlcount / int(mult.MaxGetCount)) + 1
	} else {
		count = urlcount / int(mult.MaxGetCount)
	}
	for i := 0; i < count; i++ {
		for _, v := range RequestChannes {
			fullstring = fullstring + <-v
		}
	}
	resultchan <- fullstring
}

func (mult *Multi) genGeneralChannels(urls []string, ctx context.Context, resultchan chan string, errorchan chan error) {
	var RequestChannes []chan string
	urlslen := len(urls)
	getter := make(chan string)
	if urlslen >= int(mult.MaxGetCount) {
		for i := 0; i < int(mult.MaxGetCount); i++ {
			RequestChannes = append(RequestChannes, make(chan string))
		}
		k := 0
		go mult.getforchannels(getter, RequestChannes, urlslen)
		for i := 0; i < urlslen; i += urlslen / int(mult.MaxGetCount) {
			if k+1 == int(mult.MaxGetCount) {
				go mult.httpGetCl(urls[i:], ctx, RequestChannes[k], errorchan)
				break
			} else {
				go mult.httpGetCl(urls[i:i+(urlslen/int(mult.MaxGetCount))], ctx, RequestChannes[k], errorchan)
			}
			k++
		}
	} else {
		for i := 0; i < urlslen; i++ {
			RequestChannes = append(RequestChannes, make(chan string))
		}
		go mult.getforchannels(getter, RequestChannes, urlslen)
		for i := 0; i < urlslen; i++ {
			go mult.httpGetCl(urls[i:i], ctx, RequestChannes[i], errorchan)
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
	ctxe, cansele := context.WithCancel(ctx)
	go mult.genGeneralChannels(jurls.URLS, ctxe, result, errequesr)
	select {
	case <-ctx.Done():
	case err := <-errequesr:
		cansele()
		mult.printResponeError(rw, err)
		close(handlereturn)
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
	case <-ctx.Done():
		mult.printResponeError(rw, errors.New("Server Timeout"))
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
	idleConnsClosed := make(chan struct{})
	if saveshutdown {
		go func() {
			osSig := make(chan os.Signal)
			signal.Notify(osSig, os.Interrupt)
			<-osSig
			if err := hsr.httpserv.Shutdown(context.Background()); err != nil {
				fmt.Println("HTTP SERVER SHUTDOWN ERR:" + err.Error())
			}
			close(idleConnsClosed)
		}()
	}
	if err := hsr.httpserv.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Println("HTTP SERVER ERR:" + err.Error())
		close(idleConnsClosed)
	}
	<-idleConnsClosed
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
