package rawcdp

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/websocket"
)

// Tab is used as return type of Discover.
type Tab struct {
	Description          string `json:"description"`
	DevtoolsFrontendURL  string `json:"devtoolsFrontendUrl"`
	FaviconURL           string `json:"faviconUrl"`
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// Connect dials WebSocketDebuggerURL and returns *Client.
func (s Tab) Connect(dialer *websocket.Dialer, tracer func(...interface{})) (*Client, error) {
	conn, _, err := dialer.Dial(s.WebSocketDebuggerURL, nil)
	if err != nil {
		return nil, err
	}

	return NewClient(conn, tracer), nil
}

// Discover enumerates Tabs from url.
func Discover(client *http.Client, url string) ([]Tab, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bin, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	tabs := []Tab{}
	if err = json.Unmarshal(bin, &tabs); err != nil {
		return nil, err
	}

	return tabs, nil
}

// Connect is same as Discover and Tabs[0].Connect.
func Connect(url string, tracer func(...interface{})) (*Client, error) {
	tabs, err := Discover(http.DefaultClient, url)
	if err != nil {
		return nil, err
	}

	return tabs[0].Connect(websocket.DefaultDialer, tracer)
}
