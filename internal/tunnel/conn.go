package tunnel

import (
	"context"
	"net"

	"github.com/coder/websocket"
	"github.com/xtaci/smux"
)

// NetConn adapts a WebSocket connection into a net.Conn carrying binary
// frames, which is what the smux multiplexer expects.
func NetConn(ctx context.Context, c *websocket.Conn) net.Conn {
	return websocket.NetConn(ctx, c, websocket.MessageBinary)
}

// SmuxConfig is the multiplexer configuration shared by both ends so that
// keep-alive timers match.
func SmuxConfig() *smux.Config {
	return smux.DefaultConfig()
}
