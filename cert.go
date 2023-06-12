package main

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/elazarl/goproxy"
)

const (
	certFileDefault = "~/Library/Application Support/mkcert/rootCA.pem"
	keyFileDefault  = "~/Library/Application Support/mkcert/rootCA-key.pem"
)

func setCA(caCert, caKey []byte) error {
	var err error
	if caCert == nil || caKey == nil {
		certfile, err := addHomeDir(certFileDefault)
		if err != nil {
			return err
		}
		caCert, err = os.ReadFile(certfile)
		if err != nil {
			return err
		}

		keyfile, err := addHomeDir(keyFileDefault)
		if err != nil {
			return err
		}
		caKey, err = os.ReadFile(keyfile)
		if err != nil {
			return err
		}
	}
	goproxyCa, err := tls.X509KeyPair(caCert, caKey)
	if err != nil {
		return err
	}
	if goproxyCa.Leaf, err = x509.ParseCertificate(goproxyCa.Certificate[0]); err != nil {
		return err
	}
	goproxy.GoproxyCa = goproxyCa
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCa)}
	return nil
}

func addHomeDir(path string) (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	dir := currentUser.HomeDir

	if path == "~" {
		path = dir
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(dir, path[2:])
	}

	return path, nil
}
