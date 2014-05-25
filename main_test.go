package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type closeNotifyingRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func newCloseNotifyingRecorder() *closeNotifyingRecorder {
	return &closeNotifyingRecorder{
		httptest.NewRecorder(),
		make(chan bool, 1),
	}
}

func (c *closeNotifyingRecorder) close() {
	c.closed <- true
}

func (c *closeNotifyingRecorder) CloseNotify() <-chan bool {
	return c.closed
}

func TestEventSourceHandler(t *testing.T) {
	h := NewHub()
	setup := func() {
		err := h.client.Set("existing-token", []byte("channelName"))
		if err != nil {
			t.Error("[Error] An error occured while setting up the test", err)
		}
	}
	tearDown := func() {
		_, err := h.client.Del("existing-token")
		if err != nil {
			t.Error("[Error] An error occured while tearing down the test", err)
		}
		_, err = h.client.Del("bad-token")
		if err != nil {
			t.Error("[Error] An error occured while tearing down the test", err)
		}

	}

	tests := []struct {
		Desc    string
		Handler func(http.ResponseWriter, *http.Request)
		Path    string
		Status  int
	}{
		{
			Desc:    "Access the EventSourceHandler with a bad-token",
			Handler: h.EventSourceHandler,
			Path:    ssePath + "bad-token",
			Status:  http.StatusUnauthorized,
		},
		{
			Desc:    "Access the EventSourceHandler with an existing-token",
			Handler: h.EventSourceHandler,
			Path:    ssePath + "existing-token",
			Status:  http.StatusOK,
		},
	}
	for _, test := range tests {
		fmt.Println("[Testing]", test.Desc)
		setup()
		defer tearDown()
		record := newCloseNotifyingRecorder()
		req := &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: test.Path},
		}
		go test.Handler(record, req)
		// walk around a timing issue
		// When we connect to the eventsrouce endpoint the connection is hold open
		// so we need to start the handler in a goroutine else the test is blocking
		time.Sleep(time.Second * 1)
		record.CloseNotify()

		if got, want := record.Code, test.Status; got != want {
			t.Errorf("%s: response code = %d, want %d", test.Desc, got, want)
		}
	}
}
