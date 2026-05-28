package tunnel

// TunnelPath is the WebSocket path agents use to register with the relay.
const TunnelPath = "/_tunnel"

// RegisterRequest is sent by the agent immediately after the WebSocket
// handshake, as a single JSON text message, before the multiplexed binary
// stream takes over the connection.
type RegisterRequest struct {
	Token     string `json:"token"`
	Subdomain string `json:"subdomain,omitempty"`
}

// RegisterResponse is the relay's reply to a RegisterRequest.
type RegisterResponse struct {
	OK        bool   `json:"ok"`
	URL       string `json:"url,omitempty"`
	Subdomain string `json:"subdomain,omitempty"`
	Error     string `json:"error,omitempty"`
}
