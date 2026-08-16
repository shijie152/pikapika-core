package main

import (
	"errors"
	"github.com/go-flutter-desktop/go-flutter"
	"github.com/go-flutter-desktop/go-flutter/plugin"
	"github.com/go-flutter-desktop/plugins/url_launcher"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/miguelpruivo/flutter_file_picker/go"
	"pikapika/pikapika"
	"pikapika/pikapika/database/properties"
	"strconv"
	"sync"
)

var options = []flutter.Option{
	flutter.AddPlugin(&PikapikaPlugin{}),
	flutter.AddPlugin(&file_picker.FilePickerPlugin{}),
	flutter.AddPlugin(&url_launcher.UrlLauncherPlugin{}),
}

var eventMutex = sync.Mutex{}
var eventSink *plugin.EventSink

type EventHandler struct {
}

func (s *EventHandler) OnListen(arguments interface{}, sink *plugin.EventSink) {
	eventMutex.Lock()
	defer eventMutex.Unlock()
	eventSink = sink
}

func (s *EventHandler) OnCancel(arguments interface{}) {
	eventMutex.Lock()
	defer eventMutex.Unlock()
	eventSink = nil
}

const channelName = "method"

type PikapikaPlugin struct {
}

func (p *PikapikaPlugin) InitPlugin(messenger plugin.BinaryMessenger) error {

	channel := plugin.NewMethodChannel(messenger, channelName, plugin.StandardMethodCodec{})

	channel.HandleFunc("flatInvoke", func(arguments interface{}) (interface{}, error) {
		if argumentsMap, ok := arguments.(map[interface{}]interface{}); ok {
			if method, ok := argumentsMap["method"].(string); ok {
				if params, ok := argumentsMap["params"].(string); ok {
					return pikapika.FlatInvoke(method, params)
				}
			}
		}
		return nil, errors.New("params error")
	})

	// 数据目录
	channel.HandleFunc("dataLocal", func(arguments interface{}) (interface{}, error) {
		return pikapika.AppHomeDir(), nil
	})

	// 数据迁移
	channel.HandleFunc("migrate", func(arguments interface{}) (interface{}, error) {
		return nil, nil
	})

	// 身份验证 (桌面无生物识别, 直接放行)
	channel.HandleFunc("verifyAuthentication", func(arguments interface{}) (interface{}, error) {
		return true, nil
	})

	// 字体列表
	channel.HandleFunc("fontList", func(arguments interface{}) (interface{}, error) {
		return []string{}, nil
	})

	exporting := plugin.NewEventChannel(messenger, "flatEvent", plugin.StandardMethodCodec{})
	exporting.Handle(&EventHandler{})

	pikapika.EventNotify = func(message string) {
		eventMutex.Lock()
		defer eventMutex.Unlock()
		sink := eventSink
		if sink != nil {
			sink.Success(message)
		}
	}

	// 网络状态通道
	networkChannel := plugin.NewMethodChannel(messenger, "network", plugin.StandardMethodCodec{})
	networkChannel.HandleFunc("getNetworkType", func(arguments interface{}) (interface{}, error) {
		return "wifi", nil
	})
	networkChannel.HandleFunc("getIsMobile", func(arguments interface{}) (interface{}, error) {
		return false, nil
	})

	return nil // no error
}

func (p *PikapikaPlugin) InitPluginGLFW(window *glfw.Window) error {
	window.SetSizeCallback(func(w *glfw.Window, width int, height int) {
		go func() {
			properties.SaveProperty("window_width", strconv.Itoa(width))
			properties.SaveProperty("window_height", strconv.Itoa(height))
		}()
	})
	window.SetMaximizeCallback(func(w *glfw.Window, iconified bool) {
		go func() {
			properties.SaveProperty("full_screen", strconv.FormatBool(iconified))
		}()
	})
	return nil
}
