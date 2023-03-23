package node

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/teslapatrick/openai-goproxy"
	"github.com/teslapatrick/openai-goproxy/chat"
	"github.com/valyala/fasthttp"
)

type Node struct {
	config *Config

	startCh chan struct{}
	stopCh  chan struct{}
	exitCh  chan struct{}

	ListeningServer *fasthttp.Server

	wg sync.WaitGroup
}

func New(config *Config) *Node {
	return &Node{
		config: config,
		ListeningServer: &fasthttp.Server{
			WriteTimeout: 300 * time.Second,
			ReadTimeout:  120 * time.Second,
		},
		startCh: make(chan struct{}),
		stopCh:  make(chan struct{}),
		exitCh:  make(chan struct{}),
	}
}

func (n *Node) Run() {
	go func() {
		log.Println(">>>>> Listening on", n.config.Service.Address)
		n.registerHandler()
		err := n.ListeningServer.ListenAndServe(n.config.Service.Address)
		if err != nil {
			log.Fatalln("===========> run server error: ", err)
		}
	}()

	for {
		select {
		case <-n.stopCh:
			n.ListeningServer.Shutdown()
			n.exitCh <- struct{}{}
			return
		}
	}
}

func (n *Node) registerHandler() {

	handler := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())
		switch path {
		case "/":
			fmt.Println(ctx.Request.Body())
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString(fmt.Sprintf("Hello, %s!", ctx.RemoteAddr()))
		case "/api/v1/openai/chat":
			if !ctx.IsPost() {
				ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
				return
			}
			// decode request body
			var req chat.CreateCompletionParams
			_ = json.Unmarshal(ctx.Request.Body(), &req)
			// new session
			s := openai.NewSession(n.config.API.Apikey)
			// stream request or not
			if !req.Stream {
				fmt.Println(">>>>>> normal req")
				// make normal client request
				// new client
				client := chat.NewClient(s, n.config.API.Model)
				// make request

				res, err := client.CreateCompletion(ctx, &req)
				ctx.SetContentType("application/json")
				if err != nil {
					ctx.SetStatusCode(fasthttp.StatusInternalServerError)
					ctx.SetBodyString(err.Error())
					return
				}
				d, _ := json.Marshal(res)
				ctx.SetStatusCode(fasthttp.StatusOK)
				ctx.SetBody(d)
			} else {
				// make stream client request
				client := chat.NewStreamingClient(s, n.config.API.Model)
				ctx.SetContentType("text/event-stream")
				ctx.Response.Header.Set("Cache-Control", "no-cache")
				ctx.Response.Header.Set("Connection", "keep-alive")
				ctx.Response.Header.Set("X-Accel-Buffering", "no")
				ctx.Response.Header.Set("Transfer-Encoding", "chunked")
				ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
				ctx.Response.Header.Set("Access-Control-Allow-Headers", "Cache-Control")
				ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
				// instantiate the channel
				messageChan := make(chan string)
				sessionDone := make(chan struct{})
				defer func() {
					close(messageChan)
					close(sessionDone)
					messageChan = nil
					sessionDone = nil
					messageChan = make(chan string)
					sessionDone = make(chan struct{})
					log.Printf(">>>>> Client connection is initialized")
				}()

				ctx.SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
					fmt.Printf("Time: %v\n[You]     %s\n[chatGPT] ", time.Now(), req.Messages[2].Content)

					go func() {
						client.CreateCompletion(ctx, &req, func(r *chat.CreateCompletionStreamingResponse) {
							d, _ := json.Marshal(r)
							messageChan <- string(d)
							fmt.Printf(r.Choices[0].Delta.Content)
							if r.Choices[0].FinishReason == "stop" {
								sessionDone <- struct{}{}
							}
						})
					}()

					for {
						select {
						case msg := <-messageChan:
							fmt.Fprintf(w, "data: %s\n\n", msg)
							// Flush the response.  This is only possible if
							// the repsonse supports streaming.
							w.Flush()
						case <-sessionDone:
							return
						case <-ctx.Done():
							return
						}
					}
				}))
			}

		default:
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBodyString(fmt.Sprintf("Hello, %s!", ctx.RemoteAddr()))
		}
	}
	n.ListeningServer.Handler = handler
}

func (n *Node) Wait() {
	<-n.exitCh
	log.Println(">>>>> Node stop & exit ...")
}

func (n *Node) Start() {
	n.startCh <- struct{}{}
}

func (n *Node) Stop() {
	n.stopCh <- struct{}{}
}
