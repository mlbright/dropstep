package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"

	"github.com/elazarl/dropstep/addomains"
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

	// Initialize ad domain database
	adDb := addomains.NewAdDomains()
	err = adDb.GetAdDomains()
	if err != nil {
		log.Fatalln("could not obtain ad domain list on startup", err)
	}

	go func() {
		for range adDb.Ticker.C {
			err := adDb.GetAdDomains()
			if err != nil {
				log.Printf("%v", err)
			}
			log.Println("ad domain refresh")
		}
	}()

	proxy.OnResponse().DoFunc(func(response *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		adDb.Requests += 1
		if adDb.Requests%addomains.RequestsUntilUpdate == 0 {
			adDb.GetAdDomains()
		}

		// log.Println(response.Request.Host)
		// host, _, err := net.SplitHostPort(response.Request.Host)
		// if err != nil {
		// 	log.Fatal("could not split the request network address into host and port")
		// }

		adDb.RwLock.RLock()
		_, isAdDomain := adDb.AdDomains[response.Request.Host]
		adDb.RwLock.RUnlock()

		if isAdDomain {
			fmt.Fprintf(logfile, "%s\n", response.Request.URL)
			b, err := httputil.DumpResponse(response, true)
			if err != nil {
				log.Fatal("huh?")
			}
			err = os.WriteFile("db/response", b, 0644)
			if err != nil {
				log.Fatal(err)
			}
			return goproxy.NewResponse(response.Request,
				goproxy.ContentTypeText, http.StatusAccepted,
				"Don't waste your time!")
		}
		return response
	})

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal("listen:", err)
	}

	log.Println("Starting Proxy")
	http.Serve(listener, proxy)
	log.Println("All connections closed - exit")
}
