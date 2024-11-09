package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	raft "raftkv/src"

	"github.com/fatih/structs"
	"github.com/gin-gonic/gin"
	"github.com/gookit/slog"
)

/*------------------------utils---------------------------*/

func getIPsInSubnet(subnetCIDR string) ([]net.IP, error) {
	// Parse the subnet CIDR
	_, subnet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, err
	}

	// Get the first and last IP addresses in the subnet
	startIP := subnet.IP
	endIP := make(net.IP, len(startIP))
	copy(endIP, startIP)

	// Calculate the end IP address by OR'ing with the subnet mask and adding the max range
	for i := 0; i < len(startIP); i++ {
		endIP[i] |= ^subnet.Mask[i]
	}

	// List of all IPs in the subnet
	var ipList []net.IP

	ip := startIP
	// Start iterating from the start IP to the end IP
	for !ip.Equal(endIP) {
		currentIP := make(net.IP, len(startIP))
		ip = ip.To4()
		ip[3] += 1
		copy(currentIP, ip)
		ipList = append(ipList, currentIP)
	}

	// Don't forget to include the end IP (it is the last valid IP in the subnet)
	ipList = append(ipList, endIP)

	return ipList, nil
}

func getLocalIPAndCIDR() (string, string, error) {
	// Get all network interfaces on the machine
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", "", err
	}

	// Loop over each interface
	for _, iface := range interfaces {
		// Skip interfaces that are not up (e.g., down or loopback interfaces)
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Get all addresses for this interface
		addresses, err := iface.Addrs()
		if err != nil {
			return "", "", err
		}

		// Look for the first valid (non-loopback) IPv4 address
		for _, addr := range addresses {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil &&
				!ipNet.IP.IsLoopback() {
				// Return the local IP and the CIDR of the network
				return ipNet.IP.String(), ipNet.String(), nil
			}
		}
	}

	// Return an error if no suitable address was found
	return "", "", fmt.Errorf("no suitable network interface found")
}

func getRaftPeers() ([]raft.Peer, int) {
	ip, ipcidr, _ := getLocalIPAndCIDR()
	slog.Infof("ip is %v, ipcidr is %v", ip, ipcidr)
	ips, _ := getIPsInSubnet(ipcidr)
	// slog.Printf("the ips are %v\n", ips)

	selfIP := net.ParseIP(ip)

	peers := []raft.Peer{}
	exist := make([]bool, len(ips))
	for i := range len(ips) {
		exist[i] = false
	}

	wg := sync.WaitGroup{}
	for i := 0; i < len(ips); i++ {
		wg.Add(1)
		go func(idx int, ex *bool) {
			defer wg.Done()
			url := fmt.Sprintf("http://%v:%v/ping", ips[idx].String(), 8080)
			res, err := http.Get(url)
			if err != nil {
				// fmt.Printf("get err %v\n", err)
				return
			}
			defer res.Body.Close()
			if res.StatusCode == http.StatusOK {
				*ex = true
			}
		}(i, &exist[i])
	}
	wg.Wait()
	// slog.Printf("exist: %v\n", exist)
	meId := -1
	id := 0
	for i, ip := range ips {
		if exist[i] != true {
			continue
		}
		peers = append(peers, raft.Peer{Id: id, Ip: ip, Port: 8080})
		if ip.Equal(selfIP) {
			meId = id
		}
		id += 1
	}
	// slog.Printf("peers: %v\n", peers)

	return peers, meId
}

/*------------------------utils---------------------------*/

var node *raft.Raft

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

	// Create a new Gin router
	router := gin.Default()

	router.POST("/requestVote", requestVote)
	router.POST("/appendEntries", appendEntries)
	router.GET("/nodeInfo", getNodeInfo)

	// Define a simple GET route
	router.GET("/ping", ping)

	// Define another GET route with a path parameter
	router.GET("/hello/:name", hello)

	node = raft.NewRaft()
	node.ReqSenderFunc = httpReqVoteFunc
	node.ReqVoteAddr = "/requestVote"

	node.ApdEntriesFunc = httpApdEntriesFunc
	node.ApdEntriesAddr = "/appendEntries"

	go func() {
		time.Sleep(5 * time.Second)
		slog.Info("get peer after sleep for 5 sec")
		peers, meId := getRaftPeers()
		slog.Infof("peers: %v", peers)
		node.SetPeer(peers)
		node.SetId(meId)
		node.Start()
	}()

	go node.Ticker()

	// Start the server on port 8080
	slog.Info("start run router")
	router.Run(":8080")
}
