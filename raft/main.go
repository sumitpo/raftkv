package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"

	raft "raftkv/src"

	"github.com/fatih/structs"
	"github.com/gin-gonic/gin"
	"github.com/gookit/slog"
)

/*------------------------utils---------------------------*/

func getIPsInSubnet(subnetCIDR string) ([]string, error) {
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
	var ipList []string

	// Start iterating from the start IP to the end IP
	for ip := startIP; !ip.Equal(endIP); incrementIP(ip) {
		ipList = append(ipList, ip.String())
	}

	// Don't forget to include the end IP (it is the last valid IP in the subnet)
	ipList = append(ipList, endIP.String())

	return ipList, nil
}

// Helper function to increment the IP address by 1
func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
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

	ip, ipcidr, _ := getLocalIPAndCIDR()
	fmt.Printf("ip is %v, ipcidr is %v\n", ip, ipcidr)
	ipStrs, _ := getIPsInSubnet(ipcidr)
	fmt.Printf("ips is %v\n", ipStrs)

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
