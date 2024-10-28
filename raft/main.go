package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	raft "raftkv/src"

	"github.com/fatih/structs"
	"github.com/gin-gonic/gin"
	"github.com/gookit/slog"
)

var node *raft.Raft

func osCmd(cmd string) []byte {
	fmt.Printf("in osCmd\n")
	out, err := exec.Command(cmd).Output()
	if err != nil {
		return out
	}
	fmt.Printf("out osCmd\n")
	return out
}

func lookupIp(hostname string) []raft.Peer {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, "127.0.0.11:53")
		},
	}
	ipAddresses, err := r.LookupHost(context.Background(), hostname)
	var peer []raft.Peer
	// ipAddresses, err := net.LookupIP(hostname)
	if err != nil {
		fmt.Printf("Error resolving hostname: %v\n", err)
		return []raft.Peer{}
	}
	fmt.Printf("%v\n", ipAddresses)
	/*
		for _, p := range ipAddresses {
			peer = append(peer, raft.Peer{Id: 0, Ip: p, Port: 8000})
		}
	*/
	return peer
}

func requestVote(c *gin.Context) {
	var reqVoteArgs raft.RequestVoteArgs
	var reqVoteReply raft.RequestVoteReply
	if err := c.BindJSON(&reqVoteArgs); err != nil {
		c.JSON(http.StatusBadRequest, structs.Map(&reqVoteArgs))
	}
	node.RequestVote(&reqVoteArgs, &reqVoteReply)
	c.JSON(http.StatusOK, structs.Map(&reqVoteReply))
}

func appendEntries(c *gin.Context) {
	var apdEntriesArgs raft.AppendEntriesArgs
	var apdEntriesReply raft.AppendEntriesReply
	if err := c.BindJSON(&apdEntriesArgs); err != nil {
		c.JSON(http.StatusBadRequest, structs.Map(&apdEntriesArgs))
	}
	node.AppendEntries(&apdEntriesArgs, &apdEntriesReply)
	c.JSON(http.StatusOK, structs.Map(&apdEntriesReply))
}

func getNodeInfo(c *gin.Context) {
	c.JSON(http.StatusOK, structs.Map(&node))
}

func httpReqVoteFunc(
	des string,
	args *raft.RequestVoteArgs,
	reply *raft.RequestVoteReply,
) {
	reqJson, err := json.Marshal(*args)
	if err != nil {
		slog.Errorf("error encode args to json")
		return
	}
	resp, err := http.Post(des, "application/json", bytes.NewBuffer(reqJson))
	if err != nil {
		slog.Errorf("error make post to [%v]", des)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Errorf("error read response body")
	}
	if err = json.Unmarshal(body, reply); err != nil {
		slog.Errorf("failed to decode response to json")
	}
}

func httpApdEntriesFunc(
	des string,
	args *raft.AppendEntriesArgs,
	reply *raft.AppendEntriesReply,
) {
	reqJson, err := json.Marshal(*args)
	if err != nil {
		slog.Errorf("error encode args to json")
		return
	}
	resp, err := http.Post(des, "application/json", bytes.NewBuffer(reqJson))
	if err != nil {
		slog.Errorf("error make post to [%v]", des)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Errorf("error read response body")
	}
	if err = json.Unmarshal(body, reply); err != nil {
		slog.Errorf("failed to decode response to json")
	}
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

func hello(c *gin.Context) {
	name := c.Param("name")
	c.JSON(http.StatusOK, gin.H{"message": "Hello " + name})
}

func main() {
	slog.Configure(func(logger *slog.SugaredLogger) {
		f := logger.Formatter.(*slog.TextFormatter)
		f.EnableColor = true
	})

	hostname, _ := os.Hostname()
	ips := lookupIp(hostname)
	for _, v := range ips {
		fmt.Printf("[%v] ", v.Ip.String())
	}
	fmt.Printf("\n")

	fmt.Printf("hello\n")
	fmt.Printf("%v\n", osCmd(fmt.Sprintf("nslookup %v", hostname)))
	fmt.Printf("world\n")

	// Create a new Gin router
	router := gin.Default()

	router.POST("/requestVote", requestVote)
	router.POST("/appendEntries", appendEntries)
	router.GET("/nodeInfo", getNodeInfo)

	// Define a simple GET route
	router.GET("/ping", ping)

	// Define another GET route with a path parameter
	router.GET("/hello/:name", hello)
	fmt.Printf("hello\n")

	node = raft.NewRaft()
	node.ReqSenderFunc = httpReqVoteFunc
	node.ReqVoteAddr = "/requestVote"

	node.ApdEntriesFunc = httpApdEntriesFunc
	node.ApdEntriesAddr = "/appendEntries"
	go node.Ticker()

	// Start the server on port 8080
	router.Run(":8080")
}
