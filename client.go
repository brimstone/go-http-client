package http

import (
	"bytes"
	"fmt"
	"net"
	orig "net/http"
	"strings"

	"github.com/brimstone/logger"
	"golang.org/x/net/proxy"
)

type (
	Response  = orig.Response
	Request   = orig.Request
	Transport = orig.Transport
)

var DefaultClient = &Client{}

type Client struct {
	orig.Client

	proxies []proxyInfo
}

type proxyInfo struct {
	method  proxyMethod
	address string
}

type proxyMethod string

const (
	proxySocks5 proxyMethod = "socks5"
	proxyHttps  proxyMethod = "https"
)

// Overload Get
func (c *Client) Get(url string) (*orig.Response, error) {
	fmt.Printf("proxies %#v\n", c.proxies)
	t := &Transport{}
	for _, p := range c.proxies {
		switch p.method {
		case proxySocks5:
			t.Dial = proxyViaSOCKS5("tcp", p.address, t.Dial)
		case proxyHttps:
			t.Dial = proxyViaHTTPS("tcp", p.address, t.Dial)
		default:
			return nil, fmt.Errorf("Unable to determine proxy method")
		}
		if t.Dial == nil {
			return nil, fmt.Errorf("Unable to setup 1 or more proxies")
		}

	}
	c.Client.Transport = t
	return c.Client.Get(url)
}

func Get(url string) (*orig.Response, error) {
	return DefaultClient.Get(url)
}

// Add WithSOCKS5
func (c *Client) WithSOCKS5(p string) *Client {
	d := &Client{}
	*d = *c
	d.proxies = append(d.proxies, proxyInfo{method: proxySocks5, address: p})
	return d
}

func WithSOCKS5(p string) *Client {
	return DefaultClient.WithSOCKS5(p)
}

type dial func(network string, address string) (net.Conn, error)

type dialer struct {
	dial dial
}

func (d *dialer) Dial(network, address string) (net.Conn, error) {
	return d.dial(network, address)
}

func proxyViaSOCKS5(network, proxyaddress string, forward dial) dial {
	f := &dialer{dial: forward}

	return func(network, address string) (net.Conn, error) {
		var d proxy.Dialer
		var err error
		if f.dial == nil {
			d, err = proxy.SOCKS5(network, proxyaddress, nil, nil)
		} else {
			d, err = proxy.SOCKS5(network, proxyaddress, nil, f)
		}
		if err != nil {
			return nil, err
		}
		return d.Dial(network, address)
	}
}

// Add WithHTTP
func (c *Client) WithHTTP(p string) *Client {
	d := &Client{}
	*d = *c
	d.proxies = append(d.proxies, proxyInfo{method: proxyHttps, address: p})
	return d
}

func WithHTTP(p string) *Client {
	return DefaultClient.WithHTTP(p)
}

func proxyViaHTTPS(network, proxyaddress string, forward dial) dial {
	parts := strings.Split(proxyaddress, ":")
	switch len(parts) {
	case 1:
		proxyaddress = proxyaddress + ":80"
	case 2:
		if parts[0] == "http" || parts[0] == "https" {
			proxyaddress = strings.TrimLeft(parts[1], "/") + ":80"
		}
	case 3:
		proxyaddress = strings.TrimLeft(parts[1], "/") + ":" + parts[2]
	default:
		panic("what even")
	}
	return func(network, address string) (net.Conn, error) {
		log := logger.New()
		host, _, err := net.SplitHostPort(address)

		c, err := forward("tcp", proxyaddress)
		if err != nil {
			return nil, err
		}
		log.Println("Sending connection information",
			log.Field("address", address),
		)

		fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\n", address)
		fmt.Fprintf(c, "Host: %s\r\n", host)
		fmt.Fprintf(c, "User-Agent: curl/7.72.0\r\n")
		fmt.Fprintf(c, "Proxy-Connection: Keep-Alive\r\n")
		fmt.Fprintf(c, "\r\n")

		log.Println("Waiting for response",
			log.Field("address", address),
		)
		ok11 := []byte("HTTP/1.1 200 ")
		ok10 := []byte("HTTP/1.0 200 ")
		b := make([]byte, len(ok11))
		_, err = c.Read(b)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(b, ok11) && !bytes.Equal(b, ok10) {
			return nil, fmt.Errorf("bytes don't match: %q", b)
		}
		log.Info("All good!",
			log.Field("address", address),
		)

		return c, nil
	}
}
