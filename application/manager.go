package application

import (
	"math/rand"
	"sync"
	"time"

	"github.com/kpango/fastime"
	"github.com/shift-jis/snake-missile/utilities"
	"go.uber.org/zap"
)

type ListenerFunc func(earthworm *Earthworm, decodedPacketData []int) [][]byte

type MissileManager struct {
	Properties *ProgramProperties
	WaitGroup  *sync.WaitGroup
	Logger     *zap.Logger

	ListenerFunctions map[int]ListenerFunc
	ConnectedChan     chan *Earthworm
	Earthworms        []*Earthworm

	LastActivityTime time.Time
	ActivityMutex    sync.Mutex
}

func NewMissileManager(properties *ProgramProperties) *MissileManager {
	return &MissileManager{
		Properties: properties,
		WaitGroup:  new(sync.WaitGroup),
		Logger:     utilities.MustDevelopmentLogger(),

		ListenerFunctions: make(map[int]ListenerFunc),
		ConnectedChan:     make(chan *Earthworm, 100),

		LastActivityTime: fastime.Now(),
	}
}

func (manager *MissileManager) InitializeEarthworms() error {
	proxyList, err := manager.Properties.ReadProxyList()
	if err != nil {
		return err
	}

	manager.Earthworms = make([]*Earthworm, len(proxyList))
	for index := 0; index < len(proxyList); index++ {
		manager.Earthworms[index] = NewEarthworm(manager, proxyList[index])
	}

	return nil
}

func (manager *MissileManager) InitializeListeners() {
	manager.RegisterListener([]int{54}, func(earthworm *Earthworm, decodedPayload []int) [][]byte {
		manager.Logger.Debug("Received HELLO packet", zap.String("nickname", earthworm.Nickname))
		println("Received HELLO packet " + earthworm.Nickname)

		setNicknamePacket := make([]byte, 4+len(earthworm.Nickname))
		setNicknamePacket[0] = 115
		setNicknamePacket[1] = 10
		setNicknamePacket[2] = byte(rand.Intn(42))
		setNicknamePacket[3] = byte(len(earthworm.Nickname))
		for index, char := range earthworm.Nickname {
			setNicknamePacket[4+index] = byte(char)
		}

		return [][]byte{
			utilities.DecodeSecret(decodedPayload),
			setNicknamePacket,
		}
	})
	manager.RegisterListener([]int{97}, func(earthworm *Earthworm, decodedPayload []int) [][]byte {
		manager.Logger.Debug("Received INIT packet", zap.String("nickname", earthworm.Nickname))
		println("Received INIT packet " + earthworm.Nickname)

		earthworm.IsInitialized = true
		earthworm.NeedsPing = true
		return nil
	})
	manager.RegisterListener([]int{112}, func(earthworm *Earthworm, decodedPayload []int) [][]byte {
		earthworm.NeedsPing = true
		return nil
	})
	manager.RegisterListener([]int{118}, func(earthworm *Earthworm, decodedPayload []int) [][]byte {
		earthworm.IsConnected = false
		earthworm.IsDead = true
		return nil
	})
	manager.RegisterListener([]int{115}, func(earthworm *Earthworm, decodedPayload []int) [][]byte {
		if earthworm.Identifier == 0 {
			earthworm.Identifier = utilities.DecodeIdentifier(decodedPayload)
		}

		if earthworm.Identifier == utilities.DecodeIdentifier(decodedPayload) && len(decodedPayload) >= 31 {
			earthworm.CurrentSpeed = int64((decodedPayload[12]<<8 | decodedPayload[13]) / 1e3)

			if ((((decodedPayload[18] << 16) | (decodedPayload[19] << 8) | decodedPayload[20]) / 5) > 99) || ((((decodedPayload[21] << 16) | (decodedPayload[22] << 8) | decodedPayload[23]) / 5) > 99) {
				earthworm.PositionX = ((decodedPayload[18] << 16) | (decodedPayload[19] << 8) | decodedPayload[20]) / 5
				earthworm.PositionY = ((decodedPayload[21] << 16) | (decodedPayload[22] << 8) | decodedPayload[23]) / 5
			}
		}
		return nil
	})
	manager.RegisterListener([]int{110, 103}, func(earthworm *Earthworm, decodedPayload []int) [][]byte {
		if earthworm.Identifier == utilities.DecodeIdentifier(decodedPayload) {
			earthworm.PositionX = decodedPayload[5]<<8 | decodedPayload[6]
			earthworm.PositionY = decodedPayload[7]<<8 | decodedPayload[8]

			if earthworm.HasReceivedPacket {
				distanceTravelled := int(earthworm.CurrentSpeed * (fastime.Since(fastime.Now()) - fastime.Since(earthworm.LastPacketTime)).Milliseconds() / 4)
				earthworm.UpdatePositionByAngle(distanceTravelled)
			}

			earthworm.RecordPacketReception()
		}
		return nil
	})
	manager.RegisterListener([]int{71, 78}, func(earthworm *Earthworm, decodedPayload []int) [][]byte {
		if earthworm.Identifier == utilities.DecodeIdentifier(decodedPayload) {
			earthworm.PositionX += decodedPayload[5] - 128
			earthworm.PositionY += decodedPayload[6] - 128

			if earthworm.HasReceivedPacket {
				distanceTravelled := int(earthworm.CurrentSpeed * (fastime.Since(fastime.Now()) - fastime.Since(earthworm.LastPacketTime)).Milliseconds() / 4)
				earthworm.UpdatePositionByAngle(distanceTravelled)
			}

			earthworm.RecordPacketReception()
		}
		return nil
	})
}

func (manager *MissileManager) StartConnections() {
	for _, earthworm := range manager.Earthworms {
		manager.WaitGroup.Add(1)
		go func(earthworm *Earthworm) {
			defer manager.WaitGroup.Done()
			manager.ConnectToServer(earthworm)
		}(earthworm)
	}

	manager.WaitGroup.Wait()
}

func (manager *MissileManager) ManageConnections() {
	for {
		select {
		case earthworm := <-manager.ConnectedChan:
			go func(earthworm *Earthworm) {
				go earthworm.ManageConnection(manager.ListenerFunctions)

				for earthworm.IsConnected && !earthworm.IsDead {
					manager.ActivityMutex.Lock()
					earthworm.UpdateState()
					earthworm.UpdateAngleTowardsPoint(32000, 32000)
					manager.LastActivityTime = fastime.Now()
					manager.ActivityMutex.Unlock()
					time.Sleep(time.Millisecond * 100)
				}

				println("Connection closed " + earthworm.Nickname)
				manager.ConnectToServer(earthworm)
			}(earthworm)
		default:
			if fastime.Now().Sub(manager.LastActivityTime) > 10*time.Second {
				return // Terminate ManageConnections when all worms are dead
			}
		}
	}
}

func (manager *MissileManager) ConnectToServer(earthworm *Earthworm) {
	if err := earthworm.ConnectToServer(manager.Properties.ServerAddress); err != nil {
		manager.Logger.Error("Failed to connect to server", zap.Error(err))
		return
	}

	manager.LastActivityTime = fastime.Now()
	manager.ConnectedChan <- earthworm
}

func (manager *MissileManager) RegisterListener(packetIds []int, listenerFunc ListenerFunc) {
	for _, packetId := range packetIds {
		manager.ListenerFunctions[packetId] = listenerFunc
	}
}
