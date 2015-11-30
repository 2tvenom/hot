package hot

import (
	"os"
	"os/signal"
)

type (
	Signal struct {
		source chan os.Signal
		signal os.Signal
	}
)

func NewSignal(signal os.Signal) *Signal {
	return &Signal{
		source: make(chan os.Signal),
		signal: signal,
	}
}

func (s *Signal) watch() {
	signal.Notify(s.source, s.signal)
}

func (s *Signal) Watch() chan os.Signal {
	s.watch()
	return s.source
}

func (s *Signal) WatchHandler(handler func()) {
	go func() {
		s.watch()
		<-s.source
		handler()
	}()
}
