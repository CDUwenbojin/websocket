package websocket

import "sync"

type SessionMap map[SessionID]*Session

type SessionManager struct {
	sessions       SessionMap
	mtx            sync.RWMutex
	connectHandler ConnectHandler
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(SessionMap),
	}
}

func (s *SessionManager) RegisterConnectHandler(handler ConnectHandler) {
	s.connectHandler = handler
}

func (s *SessionManager) Clean() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.sessions = SessionMap{}
}

func (s *SessionManager) Count() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return len(s.sessions)
}

func (s *SessionManager) Get(sessionId SessionID) (*Session, bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	c, ok := s.sessions[sessionId]
	return c, ok
}

func (s *SessionManager) Range(fn func(*Session)) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, v := range s.sessions {
		fn(v)
	}
}

func (s *SessionManager) Add(c *Session) {
	s.mtx.Lock()
	//defer s.mtx.Unlock()   这里在添加会话时，会处理连接函数，连接函数中有发送消息的函数，而发送消息函数会调用Get(sessionId SessionID)再一次加锁，造成死锁
	//log.Info("[websocket] add session: ", c.SessionID())
	s.sessions[c.SessionID()] = c

	s.mtx.Unlock() //在调用连接函数之前解锁

	if s.connectHandler != nil {
		s.connectHandler(c.SessionID(), true) //连接函数
	}
}

func (s *SessionManager) Remove(c *Session) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	for k, v := range s.sessions {
		if c == v {
			//log.Info("[websocket] remove session: ", c.SessionID())
			if s.connectHandler != nil {
				s.connectHandler(c.SessionID(), false)
			}
			delete(s.sessions, k)
			return
		}
	}
}
