package main

import (
	"github.com/cooldogedev/spectrum-df"
	"github.com/cooldogedev/spectrum-df/util"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	log.Formatter = &logrus.TextFormatter{ForceColors: true}

	util.RegisterPacketDecode(packet.IDText, true)
	chat.Global.Subscribe(chat.StdoutSubscriber{})
	conf, err := server.DefaultConfig().Config(log)
	if err != nil {
		log.Fatal(err)
	}

	conf.Listeners = []func(conf server.Config) (server.Listener, error){func(conf server.Config) (server.Listener, error) {
		return spectrum.NewListener(":19133", nil, nil)
	}}

	srv := conf.New()
	srv.CloseOnProgramEnd()
	srv.Listen()
	for srv.Accept(nil) {
	}
}
