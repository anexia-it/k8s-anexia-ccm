package event

import "sync"

type Configuration struct {
	// Defines how much buffer is used inside the channels that are created
	QueueSize uint8
}

type receiverList struct {
	sync.Mutex
	Receivers []Receiver
}

type Scope struct {
	sync.Mutex
	Configuration *Configuration
	subscirbers   map[string]*receiverList
}

var DefaultScope = &Scope{
	Mutex: sync.Mutex{},
	Configuration: &Configuration{
		QueueSize: 10,
	},
}

type Event struct {
	Key     string
	Payload interface{}
}
type Receiver <-chan Event
type Publisher chan<- Event
type CancelFunc func()

func (s *Scope) GetPublisher() Publisher {
	c := make(chan Event, s.Configuration.QueueSize)

	go func() {
		for e := range c {
			s.publishEvent(&e)
		}
	}()

	return c
}

func (s *Scope) publishEvent(e *Event) {
}

func (s *Scope) addSubscriber(key string, receiver Receiver) {
	list, ok := s.subscirbers[key]
	// we need to check whether we already have subscribers for this and if not add them
	if !ok {
		s.Lock()
		if _, ok := s.subscirbers[key]; !ok {
			s.subscirbers[key] = &receiverList{Mutex: sync.Mutex{}}
		}
		s.Unlock()
		s.addSubscriber(key, receiver)
		return
	}

	list.Lock()
	list.Receivers = append(list.Receivers, receiver)
	defer list.Unlock()
}

// Subscribe to events with a specific key
func (s *Scope) Subscribe(key string) Receiver {
	c := make(chan Event, s.Configuration.QueueSize)
	s.addSubscriber(key, c)
	return c
}
