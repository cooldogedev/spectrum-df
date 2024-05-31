package main

import (
	"github.com/cooldogedev/spectrum-df"
	"github.com/cooldogedev/spectrum-df/transport"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	log.Formatter = &logrus.TextFormatter{ForceColors: true}

	chat.Global.Subscribe(chat.StdoutSubscriber{})
	conf, err := server.DefaultConfig().Config(log)
	if err != nil {
		log.Fatal(err)
	}

	conf.Listeners = []func(conf server.Config) (server.Listener, error){func(conf server.Config) (server.Listener, error) {
		return spectrum.NewListener(":19133", nil, transport.NewTCP())
	}}

	srv := conf.New()
	srv.CloseOnProgramEnd()
	srv.Listen()
	for srv.Accept(nil) {
	}
}
