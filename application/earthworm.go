package application

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/kpango/fastime"
	"github.com/shift-jis/snake-missile/utilities"
	"go.uber.org/zap"
)

const (
	// RadiansToGameUnits converts radians to the game's internal angle representation.
	// The game seems to use a 0-250 scale for a full circle.
	RadiansToGameUnits = 250.0 / (2.0 * math.Pi)
)

type Earthworm struct {
	Connection *websocket.Conn
	Dialer     *websocket.Dialer
	Logger     *zap.Logger
	Mutex      sync.Mutex

	LastAngleUpdated time.Time
	LastPacketTime   time.Time
	LastPingSent     time.Time

	PreviousAngle float64
	CurrentAngle  float64
	CurrentSpeed  int64
	PositionX     int
	PositionY     int

	Nickname   string
	Identifier int

	HasReceivedPacket bool
	IsInitialized     bool
	IsConnected       bool
	NeedsPing         bool
	IsDead            bool
}

func NewEarthworm(application *MissileManager, proxyString string) *Earthworm {
	return &Earthworm{
		Dialer:   NewProxiedDialer(proxyString),
		Logger:   application.Logger,
		Nickname: fmt.Sprintf("Missile_%v", rand.Intn(99999)),
	}
}

func NewProxiedDialer(proxyString string) *websocket.Dialer {
	return &websocket.Dialer{
		Proxy: func(httpRequest *http.Request) (*url.URL, error) {
			if proxyString != "" && len(strings.TrimSpace(proxyString)) > 0 {
				return utilities.ParseProxyURL(proxyString)
			}
			return http.ProxyFromEnvironment(httpRequest)
		},
		HandshakeTimeout:  time.Second * 20,
		EnableCompression: true,
		ReadBufferSize:    8192,
	}
}

func (earthworm *Earthworm) ConnectToServer(serverAddress string) error {
	connection, response, err := earthworm.Dialer.Dial(fmt.Sprintf("ws://%s/slither", serverAddress), http.Header{
		"User-Agent":               {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"},
		"Accept-Encoding":          {"gzip, deflate"},
		"Sec-WebSocket-Extensions": {"permessage-deflate; client_max_window_bits"},
		"Sec-websocket-version":    {"13"},
		"Origin":                   {"http://slither.com"},
		"Pragma":                   {"no-cache"},
		"Cache-Control":            {"no-cache"},
	})
	if err != nil || response.StatusCode != http.StatusSwitchingProtocols {
		return err
	}

	earthworm.Mutex.Lock()
	defer earthworm.Mutex.Unlock()

	earthworm.Connection = connection
	earthworm.IsConnected = true
	earthworm.ResetStates()
	return nil
}

func (earthworm *Earthworm) ManageConnection(listenerFunctions map[int]ListenerFunc) {
	defer func(connection *websocket.Conn) {
		earthworm.Mutex.Lock()
		if earthworm.Connection != nil {
			_ = earthworm.Connection.Close()
		}
		earthworm.ResetStates()
		earthworm.Mutex.Unlock()
	}(earthworm.Connection)

	earthworm.SendPacket([]byte{99}) // HELLO PACKET
	for earthworm.Connection != nil && earthworm.IsConnected && !earthworm.IsDead {
		messageType, incomingPayload, err := earthworm.Connection.ReadMessage()
		if err != nil {
			earthworm.Logger.Error("Failed to read message, closing connection", zap.Error(err))
			earthworm.IsConnected = false
			break
		}

		if messageType != websocket.BinaryMessage || len(incomingPayload) < 3 {
			continue
		}

		decodedPayload := make([]int, len(incomingPayload))
		for index, datum := range incomingPayload {
			decodedPayload[index] = int(datum & 255)
		}

		if listenerFunc, ok := listenerFunctions[decodedPayload[2]]; ok {
			for _, outgoingPayload := range listenerFunc(earthworm, decodedPayload) {
				earthworm.SendPacket(outgoingPayload)
			}
		}
	}
}

func (earthworm *Earthworm) UpdateState() {
	if !earthworm.IsInitialized || !earthworm.IsConnected || earthworm.IsDead {
		return
	}

	if fastime.Now().Sub(earthworm.LastAngleUpdated) > 100*time.Millisecond && earthworm.PreviousAngle != earthworm.CurrentAngle {
		earthworm.SendPacket([]byte{byte(math.Floor(earthworm.CurrentAngle))})
		earthworm.PreviousAngle = earthworm.CurrentAngle
		earthworm.LastAngleUpdated = fastime.Now()
	}

	if fastime.Now().Sub(earthworm.LastPingSent) > 250*time.Millisecond && earthworm.NeedsPing {
		earthworm.SendPacket([]byte{251})
		earthworm.LastPingSent = fastime.Now()
		earthworm.NeedsPing = false
	}
}

func (earthworm *Earthworm) SendPacket(outgoingPayload []byte) {
	if earthworm.Connection == nil {
		earthworm.Logger.Warn("Attempted to send packet on a nil connection")
		return
	}

	if err := earthworm.Connection.WriteMessage(websocket.BinaryMessage, outgoingPayload); err != nil {
		earthworm.Logger.Error("Failed to send packet", zap.Error(err))
	}
}

func (earthworm *Earthworm) UpdateAngleTowardsPoint(destinationX, destinationY int) {
	earthworm.Mutex.Lock()
	defer earthworm.Mutex.Unlock()

	if !earthworm.IsInitialized || !earthworm.IsConnected || earthworm.IsDead {
		return
	}

	deltaX := float64(destinationX - earthworm.PositionX)
	deltaY := float64(destinationY - earthworm.PositionY)

	gameAngle := math.Atan2(deltaY, deltaX) * RadiansToGameUnits
	if gameAngle < 0 {
		gameAngle += 250
	}

	earthworm.CurrentAngle = math.Round(gameAngle)
}

func (earthworm *Earthworm) UpdatePositionByAngle(distance int) {
	earthworm.Mutex.Lock()
	defer earthworm.Mutex.Unlock()

	angleRadians := earthworm.CurrentAngle / RadiansToGameUnits
	earthworm.PositionX += int(math.Cos(angleRadians) * float64(distance))
	earthworm.PositionY += int(math.Sin(angleRadians) * float64(distance))
}

func (earthworm *Earthworm) ResetStates() {
	earthworm.Identifier = 0
	earthworm.IsInitialized = false
	earthworm.HasReceivedPacket = false
	earthworm.IsDead = false
	earthworm.NeedsPing = false
}

func (earthworm *Earthworm) RecordPacketReception() {
	earthworm.Mutex.Lock()
	defer earthworm.Mutex.Unlock()

	earthworm.LastPacketTime = fastime.Now()
	earthworm.HasReceivedPacket = true
}
