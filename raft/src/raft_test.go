package raft

import (
	"testing"
)

func Test1A(t *testing.T) {
	node := NewRaft()
	// defer node.Shutdown()
	go node.Ticker()

	state := node.GetState()
	if state != "candidate" {
		t.Error("start state is not candidate")
	}
	if node.IsAlive() == false {
		t.Error("node is alive before shutdown")
	}
	node.Shutdown()
	if node.IsAlive() == true {
		t.Error("node is alive after shutdown")
	}
}
