package raft

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gookit/slog"
	"golang.org/x/exp/rand"
)

type RoleState int32

const (
	leader RoleState = iota
	follower
	candidate
)

func roleFmt(role RoleState) string {
	switch role {
	case leader:
		return "leader"
	case candidate:
		return "candidate"
	case follower:
		return "follower"
	default:
		return "none"
	}
}

const (
	minTime     int64 = 50
	randTimeDur int64 = 300
	heartbeat   int64 = 40
)

type Peer struct {
	Id   int
	Ip   net.IP
	Port int
}

type Raft struct {
	Alive       atomic.Bool `json:"alive"`
	LeadId      int         `json:"leadId"`
	CurrentTerm int         `json:"currentTerm"`
	Role        RoleState   `json:"role"`
	timer       *time.Timer `json:"-"`
	Me          int         `json:"me"`
	Peers       []Peer      `json:"peers"`

	// send function
	ReqVoteAddr   string
	ReqSenderFunc func(des string, args *RequestVoteArgs, reply *RequestVoteReply)

	ApdEntriesAddr string
	ApdEntriesFunc func(des string, args *AppendEntriesArgs, reply *AppendEntriesReply)
}

func NewRaft() *Raft {
	tmp := Raft{
		LeadId:      -1,
		CurrentTerm: 0,
		Role:        candidate,
		timer:       time.NewTimer(time.Duration(1)),

		ReqVoteAddr:   "/RequestVote",
		ReqSenderFunc: func(des string, args *RequestVoteArgs, reply *RequestVoteReply) {},
	}
	tmp.Shutdown()
	return &tmp
}

func (rf *Raft) SetPeer(peers []Peer) {
	for _, p := range peers {
		rf.Peers = append(rf.Peers, p)
	}
}

func (rf *Raft) SetId(me int) {
	rf.Me = me
}

type RequestVoteArgs struct {
	Term        int `json:"term"`
	CandidateId int `json:"candidateId"`
}

type RequestVoteReply struct {
	Term        int  `json:"term"`
	VoteGranted bool `json:"voteGranted"`
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	if rf.Role == leader || rf.Role == follower {
		slog.Debugf(
			"sth must be fucked up, node [%v:%v] get RequestVote from node [%v]",
			rf.Me,
			roleFmt(rf.Role),
			args.CandidateId,
		)
		reply.VoteGranted = false
		return
	}
	if rf.CurrentTerm > args.Term {
		slog.Infof(
			"node [%v:%v] term is bigger than node [%v], refuse RequestVote",
			rf.Me,
			rf.CurrentTerm,
			args.CandidateId,
		)
		reply.VoteGranted = false
		return
	}
}

func (rf *Raft) sendRequestVote(
	id int,
	args *RequestVoteArgs,
	reply *RequestVoteReply,
) {
	peer := rf.Peers[id]
	addr := fmt.Sprintf(
		"http://%v:%v%v",
		peer.Ip.String(),
		peer.Port,
		rf.ReqVoteAddr,
	)
	rf.ReqSenderFunc(addr, args, reply)
}

type AppendEntriesArgs struct {
	term     int
	leaderId int
}

type AppendEntriesReply struct {
	term    int
	success bool
}

func (rf *Raft) AppendEntries(
	args *AppendEntriesArgs,
	reply *AppendEntriesReply,
) {
	if rf.Role == leader {
		slog.Warnf(
			"something must be fucked up, leader [%v] get AppendEntries from [%v]",
			rf.Me,
			args.leaderId,
		)
		return
	}

	// reset the timer
	rf.timer.Reset(
		time.Duration(minTime+rand.Int63()%randTimeDur) * time.Millisecond,
	)
}

func (rf *Raft) sendAppendEntries(
	id int,
	args *AppendEntriesArgs,
	reply *AppendEntriesReply,
) {
	peer := rf.Peers[id]
	addr := fmt.Sprintf(
		"http://%v:%v%v",
		peer.Ip.String(),
		peer.Port,
		rf.ApdEntriesAddr,
	)
	rf.ApdEntriesFunc(addr, args, reply)
}

func (rf *Raft) Shutdown() {
	rf.Alive.Store(false)
}

func (rf *Raft) Start() {
	rf.Alive.Store(true)
}

func (rf *Raft) Ticker() {
	slog.Infof("into ticker")
	for rf.Alive.Load() == false {
	}
	slog.Infof("server [%v] is Alive, role is [%v]", rf.Me, roleFmt(rf.Role))
	for rf.Alive.Load() == true {
		switch rf.Role {
		case leader:
			time.Sleep(time.Duration(heartbeat))

			args := &AppendEntriesArgs{}
			replys := make([]AppendEntriesReply, len(rf.Peers))
			var wg sync.WaitGroup
			for k := 0; k < len(rf.Peers); k++ {
				if k == rf.Me {
					continue
				}
				wg.Add(1)
				go func(id int, reply *AppendEntriesReply) {
					rf.sendAppendEntries(id, args, reply)
					wg.Done()
				}(k, &replys[k])
			}
			wg.Wait()
		case candidate:
			fallthrough
		case follower:
			// reset the timer
			rf.timer.Reset(
				time.Duration(
					minTime+rand.Int63()%randTimeDur,
				) * time.Millisecond,
			)
			// follower will never get timer up
			<-rf.timer.C
			// when the timer is up, follower switch to candidate
			if rf.Role == follower {
				slog.Debugf("node [%v] from follower to candidate", rf.Me)
				rf.Role = candidate
			}

			// increase the term of current node
			rf.CurrentTerm += 1
			// send request vote for leader selection
			args := &RequestVoteArgs{}
			args.Term = rf.CurrentTerm
			args.CandidateId = rf.Me
			replys := make([]RequestVoteReply, len(rf.Peers))
			var wg sync.WaitGroup
			for k := 0; k < len(rf.Peers); k++ {
				if k == rf.Me {
					continue
				}
				wg.Add(1)
				go func(id int, reply *RequestVoteReply) {
					rf.sendRequestVote(id, args, reply)
					wg.Done()
				}(k, &replys[k])
			}
			wg.Wait()

			voteCount := 1
			for k, v := range replys {
				if k == rf.Me {
					continue
				}
				if v.VoteGranted == true {
					voteCount += 1
				}
			}
			if voteCount > len(rf.Peers)/2+1 {
				rf.Role = leader
			}
		}
	}
}

/*
* get infomation from node
 */

func (rf *Raft) GetState() string {
	return roleFmt(rf.Role)
}

func (rf *Raft) IsAlive() bool {
	if rf.Alive.Load() {
		return true
	} else {
		return false
	}
}
