package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/elazarl/goproxy"
	"github.com/google/uuid"
	"github.com/mlbright/dropstep/addomains"
)

const (
	logFileName     = "ad-traffic.log"
	certFileDefault = "~/Library/Application Support/mkcert/rootCA.pem"
	keyFileDefault  = "~/Library/Application Support/mkcert/rootCA-key.pem"
)

func main() {
	proxy := goproxy.NewProxyHttpServer()

	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	addr := flag.String("l", ":8080", "on which address should the proxy listen")
	certfile := flag.String("c", certFileDefault, "TLS CA certificate file")
	keyfile := flag.String("k", keyFileDefault, "TLS CA certificate key file")
	flag.Parse()

	cf, err := addHomeDir(*certfile)
	if err != nil {
		log.Fatalln("could not add home dir to ", *certfile)
	}

	caCert, err := os.ReadFile(cf)
	if err != nil {
		log.Fatalln(err)
	}

	kf, err := addHomeDir(*keyfile)
	if err != nil {
		log.Fatalln("could not add home dir to ", *keyfile)
	}

	caKey, err := os.ReadFile(kf)
	if err != nil {
		log.Fatalln(err)
	}

	err = setCA(caCert, caKey)
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

	err = os.MkdirAll("db", 0755)
	if err != nil {
		log.Fatalln("could not create db directory", err)
	}

	proxy.OnResponse().DoFunc(func(response *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		adDb.Requests += 1
		if adDb.Requests%addomains.RequestsUntilUpdate == 0 {
			adDb.GetAdDomains()
		}

		adDb.RwLock.RLock()
		_, isAdDomain := adDb.AdDomains[response.Request.Host]
		adDb.RwLock.RUnlock()

		if isAdDomain {
			fmt.Fprintf(logfile, "%s\n", response.Request.URL)
			b, err := httputil.DumpResponse(response, true)
			if err != nil {
				log.Fatalln("error dumping response", err)
			}

			uniqueId := uuid.NewString()
			uniqueDir := filepath.Join("db", uniqueId[0:2], uniqueId[2:4])

			err = os.MkdirAll(uniqueDir, 0755)
			if err != nil {
				log.Fatalf("could not create unique directory %s: %v\n", uniqueDir, err)
			} else {
				log.Printf("created %s if it did not exist", uniqueDir)
			}

			err = os.WriteFile(filepath.Join(uniqueDir, uniqueId), b, 0644)
			if err != nil {
				log.Fatal(err)
			}

			return goproxy.NewResponse(response.Request, goproxy.ContentTypeText, http.StatusOK, "your ad here")
		}

		return response
	})

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal("listen:", err)
	}

	log.Println("starting proxy ...")
	http.Serve(listener, proxy)
}
