package golangutil

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"errors"
	"log"
)

const (
	minRead = 16 * 1024 // 16kb
)
const (
	MIMEJSON              = "application/json"
	MIMEHTML              = "text/html"
	MIMEXML               = "application/xml"
	MIMEXML2              = "text/xml"
	MIMEPlain             = "text/plain"
	MIMEPOSTForm          = "application/x-www-form-urlencoded"
	MIMEMultipartPOSTForm = "multipart/form-data"
	MIMEPROTOBUF          = "application/x-protobuf"
	MIMEMSGPACK           = "application/x-msgpack"
	MIMEMSGPACK2          = "application/msgpack"
)

type Config struct {
	MaxIdleConnsPerHost int
	Dial                time.Duration
	Timeout             time.Duration
	KeepAlive           time.Duration
	IdleConnectTimeout  time.Duration
}

type HttpClient struct {
	conf      *Config
	client    *http.Client
	dialer    *net.Dialer
	transport *http.Transport
}

// NewHTTPClient returns a new instance of httpClient
func NewHTTPClient(c *Config) *HttpClient {
	dialer := &net.Dialer{
		Timeout:   time.Duration(c.Dial),
		KeepAlive: time.Duration(c.KeepAlive),
	}
	transport := &http.Transport{
		DialContext:         dialer.DialContext,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost: c.MaxIdleConnsPerHost,
		IdleConnTimeout:     c.IdleConnectTimeout,
	}
	return &HttpClient{
		conf: c,
		client: &http.Client{
			Transport: transport,
			Timeout:   c.Timeout,
		},
	}
}

// Get makes a HTTP GET request to provided URL with context passed in
func (c *HttpClient) Get(ctx context.Context, url string, headers http.Header) (resp []byte, err error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		err = errors.New("GET - request creation failed:" + err.Error())
		return
	}

	request.Header = headers

	return c.Request(ctx, request)
}

// Post makes a HTTP POST request to provided URL with context passed in
func (c *HttpClient) Post(ctx context.Context, url, contentType string, headers http.Header, param interface{}) (resp []byte, err error) {
	request, err := http.NewRequest(http.MethodPost, url, reqBody(contentType, param))
	if err != nil {
		err = errors.New("GET - request creation failed:" + err.Error())
		return
	}
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Content-Type", contentType)
	request.Header = headers

	return c.Request(ctx, request)
}

// Put makes a HTTP PUT request to provided URL with context passed in
func (c *HttpClient) Put(ctx context.Context, url, contentType string, headers http.Header, param interface{}) (resp []byte, err error) {
	request, err := http.NewRequest(http.MethodPut, url, reqBody(contentType, param))
	if err != nil {
		err = errors.New("GET - request creation failed:" + err.Error())
		return
	}

	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Content-Type", contentType)
	request.Header = headers

	return c.Request(ctx, request)
}

// Patch makes a HTTP PATCH request to provided URL with context passed in
func (c *HttpClient) Patch(ctx context.Context, url, contentType string, headers http.Header, param interface{}) (resp []byte, err error) {
	request, err := http.NewRequest(http.MethodPatch, url, reqBody(contentType, param))
	if err != nil {
		err = errors.New("GET - request creation failed:" + err.Error())
		return
	}

	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Content-Type", contentType)
	request.Header = headers

	return c.Request(ctx, request)
}

// Delete makes a HTTP DELETE request to provided URL with context passed in
func (c *HttpClient) Delete(ctx context.Context, url, contentType string, headers http.Header, param interface{}) (resp []byte, err error) {
	request, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		err = errors.New("GET - request creation failed:" + err.Error())
		return
	}

	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Content-Type", contentType)
	request.Header = headers

	return c.Request(ctx, request)
}

func (c *HttpClient) Request(ctx context.Context, req *http.Request) (resp []byte, err error) {
	var (
		response *http.Response
		cancel   func()
	)
	ctx, cancel = context.WithTimeout(ctx, time.Duration(c.conf.Timeout))
	defer cancel()
	response, err = c.client.Do(req.WithContext(ctx))
	if err != nil {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		}
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusInternalServerError {
		log.Println("StatusInternalServerError - Status Internal ServerError error(%v)", err)
		return
	}
	if response.Header.Get("Content-Encoding") == "gzip" {
		compressedReader, e := gzip.NewReader(response.Body)
		if e != nil {
			err = e
			return
		}
		resp, err = ioutil.ReadAll(compressedReader)
	} else {
		resp, err = ioutil.ReadAll(response.Body)
	}
	return
}

func reqBody(contentType string, param interface{}) (body io.Reader) {
	var (
		err error
	)
	if contentType == MIMEPOSTForm {
		enc, ok := param.(string)
		if ok {
			body = strings.NewReader(enc)
		}
	}
	if contentType == MIMEJSON {
		buff := new(bytes.Buffer)
		err = json.NewEncoder(buff).Encode(param)
		if err != nil {
			log.Printf("failed to marshal user payload: %v", err)
			return
		}
		body = buff
	}
	return
}

func readAll(r io.Reader, capacity int64) (b []byte, err error) {
	buf := bytes.NewBuffer(make([]byte, 0, capacity))
	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()
	_, err = buf.ReadFrom(r)
	return buf.Bytes(), err
}
