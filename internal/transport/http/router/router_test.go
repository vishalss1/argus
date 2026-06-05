package router

import (
	"context"
	"net/http/httptest"
	"testing"

	gorilla "github.com/gorilla/websocket"
	transportws "github.com/vishalss1/argus/internal/transport/websocket"
)

func TestWebSocketRouteUpgradesThroughMiddleware(t *testing.T) {
	hub := transportws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	server := httptest.NewServer(New(nil, nil, nil, nil, nil, nil, nil, nil, nil, transportws.NewHandler(hub)))
	defer server.Close()

	url := "ws" + server.URL[len("http"):] + "/ws"
	conn, _, err := gorilla.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()
}
