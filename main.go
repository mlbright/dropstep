package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"

	"github.com/elazarl/goproxy"
)

const (
	logFileName = "traffic.log"
)

func main() {
	proxy := goproxy.NewProxyHttpServer()

	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	addr := flag.String("l", ":8080", "on which address should the proxy listen")
	flag.Parse()

	err := setCA(nil, nil)
	if err != nil {
		log.Fatal("could not handle local certs", err)
	}

	proxy.Verbose = *verbose

	logfile, err := os.Create(logFileName)
	if err != nil {
		log.Fatal("could not create log file", err)
	}

	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).HandleConnect(goproxy.AlwaysMitm)

	proxy.OnResponse().DoFunc(func(response *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		fmt.Fprintf(logfile, "Hellow? %s\n", response.Request.URL)
		return response
	})

	proxy.OnRequest(goproxy.ReqHostIs("www.cbc.ca")).
		HijackConnect(func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
			defer func() {
				if e := recover(); e != nil {
					ctx.Logf("error connecting to remote: %v", e)
					client.Write([]byte("HTTP/1.1 500 Cannot reach destination\r\n\r\n"))
				}
				client.Close()
			}()

			fmt.Fprintf(logfile, "%s\n", req.URL.String())

			clientBuf := bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client))

			remote, err := net.Dial("tcp", req.URL.Host)
			if err != nil {
				log.Fatal("could not dial the remote host")
			}

			client.Write([]byte("HTTP/1.1 200 Ok\r\n\r\n"))

			remoteBuf := bufio.NewReadWriter(bufio.NewReader(remote), bufio.NewWriter(remote))

			for {
				request, err := http.ReadRequest(clientBuf.Reader)
				if err != nil {
					log.Fatal("could not read request")
				}

				err = request.Write(remoteBuf)
				if err != nil {
					log.Fatal("could not write the request to the buffer")
				}

				err = remoteBuf.Flush()
				if err != nil {
					log.Fatal("could not flush the remote buffer")
				}

				response, err := http.ReadResponse(remoteBuf.Reader, request)
				if err != nil {
					log.Fatal("could not read response from remote buffer")
				}

				err = response.Write(clientBuf.Writer)
				if err != nil {
					log.Fatal("could not write to the client buffer")
				}

				err = clientBuf.Flush()
				if err != nil {
					log.Fatal("could not flush client buffer")
				}
			}
		})

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal("listen:", err)
	}

	log.Println("Starting Proxy")
	http.Serve(listener, proxy)
	log.Println("All connections closed - exit")
}
