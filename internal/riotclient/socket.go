package riotclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/coder/websocket"
)

// Subscribe registers handler for eventName, for example
// OnJsonApiEvent_chat_v4_presences. Subscriptions survive reconnects.
func (c *Client) Subscribe(eventName string, handler Handler) error {
	if eventName == "" {
		return errors.New("riotclient: empty event name")
	}
	if handler == nil {
		return errors.New("riotclient: nil handler")
	}

	// The registry and the socket move together, so a reconnect replaying
	// subscriptions cannot also send a frame for this name.
	c.subMu.Lock()
	defer c.subMu.Unlock()

	// A second handler for a name the socket already carries needs no frame;
	// subscribing twice would double every event.
	if isNew := c.subs.add(eventName, handler); !isNew {
		return nil
	}

	conn, ok := c.currentConn()
	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(c.baseContext(), c.opts.RequestTimeout)
	defer cancel()
	if err := sendSubscribe(ctx, conn, eventName); err != nil {
		c.subs.removeLast(eventName)
		return err
	}
	return nil
}

// dial opens the WAMP event socket. Basic auth goes in a header rather than
// the URI, where a password's reserved characters would need escaping.
func (c *Client) dial(ctx context.Context, creds Credentials) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, c.opts.RequestTimeout)
	defer cancel()

	header := http.Header{}
	header.Set("Authorization", creds.AuthHeader())

	conn, resp, err := websocket.Dial(ctx, creds.WebSocketURL(), &websocket.DialOptions{
		HTTPClient:   c.http,
		HTTPHeader:   header,
		Subprotocols: []string{"wamp"},
	})
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("riotclient: websocket dial: %w", err)
	}

	conn.SetReadLimit(c.opts.ReadLimit)
	return conn, nil
}

// replaySubscriptions re-sends every registered subscription. The Riot
// Client keeps no state across sockets, so a reconnect loses all of them.
func (c *Client) replaySubscriptions(ctx context.Context, conn *websocket.Conn) error {
	for _, name := range c.subs.names() {
		if err := sendSubscribe(ctx, conn, name); err != nil {
			return err
		}
	}
	return nil
}

func sendSubscribe(ctx context.Context, conn *websocket.Conn, name string) error {
	frame, err := json.Marshal([]any{opcodeSubscribe, name})
	if err != nil {
		return fmt.Errorf("riotclient: encoding a subscribe frame: %w", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, frame); err != nil {
		return fmt.Errorf("riotclient: subscribing to %s: %w", name, err)
	}
	return nil
}

// listen pumps frames until the socket drops or ctx is canceled. It owns
// closing closed, which is how Run learns the connection is gone.
func (c *Client) listen(ctx context.Context, conn *websocket.Conn, closed chan struct{}) {
	defer close(closed)
	defer c.connected.Store(false)

	for {
		kind, raw, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() == nil {
				c.logger.Warn().Err(err).Msg("The Riot Client websocket dropped")
			}
			return
		}
		if kind != websocket.MessageText {
			continue
		}
		c.subs.dispatch(raw, c.logger)
	}
}

func (c *Client) currentConn() (*websocket.Conn, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.conn, c.conn != nil
}
