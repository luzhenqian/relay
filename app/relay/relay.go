package relay

import (
	"fmt"
	"iot-sdk-go/sdk/device"
	"net"
	"time"

	"github.com/pkg/errors"
)

// Relay 继电器设备
type Relay struct {
	Instance *device.Device

	SubDeviceID uint16
	Conn        net.Conn

	OnlineTime string

	middlewares []Middleware
	outputState OutputStates
	inputState  InputStates
	th          TemperatureAndHumidity
	keepAlive   time.Duration

	OfflineCallbackFn func(relay *Relay)
	closed            chan bool
}

type Middleware func(*Relay, Data) Data

// OutputStates 输出状态集合
type OutputStates []OutputState

// OutputState 输出状态
type OutputState struct {
	Route uint8
	Value uint8
}

// InputStates 输入状态集合
type InputStates []InputState

// InputState 输入状态
type InputState struct {
	Route uint8
	Value uint8
}

// TemperatureAndHumidity 温湿度
type TemperatureAndHumidity struct {
	Temperature float64 // 温度
	Humidity    float64 //湿度
}

// GetPropertyFnMap 根据属性类型获取属性的方法集合
type GetPropertyFnMap map[PropertyType]GetPropertyFn

// GetPropertyFn 不同属性类型的获取属性方法
type GetPropertyFn func() Property

// Property 属性
type Property interface{}

// Option 继电器配置
type Option func(*Relay)

// OfflineCallback 离线回调配置
func OfflineCallback(cb func(relay *Relay)) Option {
	return func(r *Relay) {
		r.OfflineCallbackFn = cb
	}
}

// Middlewares 中间件配置
func Middlewares(middlewares ...Middleware) Option {
	return func(r *Relay) {
		r.middlewares = middlewares
	}
}

// New 创建继电器实例
func New(DeviceInstance *device.Device, conn net.Conn, subDeviceID uint16, keepAlive time.Duration, options ...Option) *Relay {
	relay := &Relay{
		Instance:    DeviceInstance,
		Conn:        conn,
		SubDeviceID: subDeviceID,
		outputState: OutputStates{},
		inputState:  InputStates{},
		th:          TemperatureAndHumidity{},
		keepAlive:   keepAlive,
		closed:      make(chan bool),
		OnlineTime:  time.Now().Format("2006-01-02 15:04:05"),
	}
	for _, option := range options {
		option(relay)
	}
	return relay
}

// Init 初始化资源
func (r *Relay) Init() error {
	// 流读取循环
	if err := r.ReadLoop(13); err != nil {
		return errors.Wrap(err, "init relay failed")
	}
	// 主动询问状态循环
	wfs := []WriteFn{
		{
			fn: r.InquiryTH,
			d:  r.keepAlive,
		},
		{
			fn: r.InquiryInputState,
			d:  r.keepAlive,
		},
	}

	if err := r.WriteLoop(wfs); err != nil {
		return err
	}
	return nil
}

// Use 使用中间件
func (r *Relay) Use(fns ...Middleware) {
	r.middlewares = append(r.middlewares, fns...)
}

// Online 上线
func (r *Relay) Online(stateTypes []PropertyType) error {
	fmt.Printf("%v 设备 %d 上线\n", time.Now().Format("2006-01-02 15:04:05"), r.SubDeviceID)
	if err := r.Init(); err != nil {
		return err
	}
	r.AutoPostProperty(stateTypes)
	return nil
}

// Offline 下线
func (r *Relay) Offline() {
	fmt.Printf("%v 设备 %d 下线\n", time.Now().Format("2006-01-02 15:04:05"), r.SubDeviceID)
	r.Conn.Close()
	if !isClosed(r.closed) {
		close(r.closed)
	}
	if r.OfflineCallbackFn != nil {
		r.OfflineCallbackFn(r)
	}
}

func isClosed(ch <-chan bool) bool {
	select {
	case <-ch:
		return true
	default:
	}
	return false
}
