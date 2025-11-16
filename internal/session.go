package internal

import (
	"fmt"
	"math/rand/v2"
)

type Session struct {
	id int64
}

// GenerateSession creates a new session with a random numeric identifier.
// The session is used to generate unique container names and branch names.
func GenerateSession() Session {
	return Session{id: rand.Int64N(10000)}
}

// String returns the string representation of the session, equivalent to calling ID().
func (s Session) String() string {
	return string(s.ID())
}

// ID returns the session identifier in the format "contagent-<number>".
// This is used as the Docker container name.
func (s Session) ID() SessionID {
	return SessionID(fmt.Sprintf("contagent-%d", s.id))
}

// Branch returns the Git branch name in the format "contagent/<number>".
// This branch is created in the container's repository for isolated work.
func (s Session) Branch() string {
	return fmt.Sprintf("contagent/%d", s.id)
}
