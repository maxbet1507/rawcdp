package rawcdp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/maxbet1507/channels"
)

// Error contains decoded values from cdp message.
type Error struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

func (s Error) Error() string {
	if s.Message != "" {
		return s.Message
	}
	return strconv.FormatInt(s.Code, 10)
}

// Client is wrapper of client for cdp WebSocket.
type Client struct {
	id        int64
	methods   map[int64]chan<- interface{}
	listeners map[string]channels.Hub
	lock      chan struct{}
	conn      *websocket.Conn
}

func (s *Client) prepareMethod(ch chan<- interface{}) int64 {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()

	id := func() int64 {
		for {
			if _, ok := s.methods[s.id]; !ok {
				return s.id
			}
			s.id++
		}
	}()

	s.id++
	s.methods[id] = ch

	return id
}

func (s *Client) cleanupMethod(id int64) chan<- interface{} {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()

	if ch, ok := s.methods[id]; ok {
		delete(s.methods, id)
		return ch
	}
	return nil
}

// Call means calling cdp method, and encode params and decode result.
func (s *Client) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	ch := make(chan interface{}, 1)
	defer close(ch)

	id := s.prepareMethod(ch)
	defer s.cleanupMethod(id)

	req := struct {
		ID     int64       `json:"id"`
		Method string      `json:"method"`
		Params interface{} `json:"params,omitempty"`
	}{
		ID:     id,
		Method: method,
		Params: params,
	}
	reqbin, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if err := s.conn.WriteMessage(websocket.TextMessage, reqbin); err != nil {
		return err
	}

	select {
	case resbin, ok := <-ch:
		if !ok {
			return fmt.Errorf("channel closed")
		}
		if result == nil {
			return nil
		}

		res := struct {
			Result interface{} `json:"result"`
			Error  *Error      `json:"error,omitempty"`
		}{
			Result: result,
		}
		if err := json.Unmarshal(resbin.([]byte), &res); err != nil {
			return err
		}

		if res.Error != nil {
			return *res.Error
		}
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

// Listener awaits method from server, and decodes params.
type Listener func(ctx context.Context, params interface{}) error

// Canceler terminates awaiting method from server.
type Canceler func()

func (s *Client) addListener(method string) (<-chan interface{}, func()) {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()

	hub, ok := s.listeners[method]
	if !ok {
		hub = channels.PubSub()
		s.listeners[method] = hub
	}

	return hub.Subscribe(false)
}

func (s *Client) getListener(method string) channels.Hub {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()

	if hub, ok := s.listeners[method]; ok {
		return hub
	}
	return nil
}

// Listen returns Listener of method, and that Canceler.
func (s *Client) Listen(method string) (Listener, Canceler) {
	sub, canceler := s.addListener(method)

	listener := func(ctx context.Context, params interface{}) error {
		select {
		case bin, ok := <-sub:
			if !ok {
				return fmt.Errorf("channel closed")
			}
			if params == nil {
				return nil
			}

			val := struct {
				Params interface{} `json:"params"`
			}{
				Params: params,
			}
			return json.Unmarshal(bin.([]byte), &val)

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return listener, canceler
}

// Close closes connections.
func (s *Client) Close() error {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()

	for _, ch := range s.methods {
		close(ch)
	}
	s.methods = map[int64]chan<- interface{}{}

	for _, hub := range s.listeners {
		close(hub)
	}
	s.listeners = map[string]channels.Hub{}

	return s.conn.Close()
}

func (s *Client) proc(tracer func(...interface{})) error {
	for {
		_, resbin, err := s.conn.ReadMessage()
		if err != nil {
			return err
		}

		tracer(string(resbin))

		res := struct {
			ID     *int64  `json:"id,omitempty"`
			Method *string `json:"method,omitempty"`
		}{}
		if err := json.Unmarshal(resbin, &res); err != nil {
			return err
		}

		if res.ID != nil {
			if ch := s.cleanupMethod(*res.ID); ch != nil {
				ch <- resbin
			}
		} else if res.Method != nil {
			if hub := s.getListener(*res.Method); hub != nil {
				hub <- resbin
			}
		}
	}
}

// NewClient returns initialized Client.
func NewClient(conn *websocket.Conn, tracer func(...interface{})) *Client {
	if tracer == nil {
		tracer = func(...interface{}) {}
	}

	client := &Client{
		methods:   map[int64]chan<- interface{}{},
		listeners: map[string]channels.Hub{},
		lock:      make(chan struct{}, 1),
		conn:      conn,
	}
	go func() {
		if err := client.proc(tracer); err != nil {
			tracer(err)
		}
	}()

	return client
}
