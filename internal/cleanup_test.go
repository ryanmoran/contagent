package internal

import (
	"errors"
	"testing"
)

func TestCleanupManager_Execute_LIFO_Order(t *testing.T) {
	m := NewCleanupManager()
	var order []string

	m.Add("first", func() error {
		order = append(order, "first")
		return nil
	})
	m.Add("second", func() error {
		order = append(order, "second")
		return nil
	})
	m.Add("third", func() error {
		order = append(order, "third")
		return nil
	})

	m.Execute()

	if len(order) != 3 {
		t.Fatalf("expected 3 cleanups, got %d", len(order))
	}
	if order[0] != "third" || order[1] != "second" || order[2] != "first" {
		t.Errorf("expected LIFO order [third, second, first], got %v", order)
	}
}

func TestCleanupManager_Execute_ContinuesOnError(t *testing.T) {
	m := NewCleanupManager()
	var executed []string

	m.Add("first", func() error {
		executed = append(executed, "first")
		return nil
	})
	m.Add("second", func() error {
		executed = append(executed, "second")
		return errors.New("second failed")
	})
	m.Add("third", func() error {
		executed = append(executed, "third")
		return nil
	})

	m.Execute()

	if len(executed) != 3 {
		t.Fatalf("expected all 3 cleanups to execute, got %d", len(executed))
	}
	if executed[0] != "third" || executed[1] != "second" || executed[2] != "first" {
		t.Errorf("expected all cleanups in LIFO order, got %v", executed)
	}
}

func TestCleanupManager_Execute_EmptyManager(t *testing.T) {
	m := NewCleanupManager()
	m.Execute()
}
