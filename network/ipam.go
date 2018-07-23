package network

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"path"
	"strings"
)

const ipamDefaultAllocatorPath = "/var/run/mydocker/network/ipam/subnet.json"

type IPAM struct {
	SubnetAllocatorPath string
	Subnets             *map[string]string
}

// An IPAM singleton instance
var ipAllocator = &IPAM{
	SubnetAllocatorPath: ipamDefaultAllocatorPath,
}

func (ipam *IPAM) load() error {
	if _, err := os.Stat(ipam.SubnetAllocatorPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}
	subnetConfigFile, err := os.Open(ipam.SubnetAllocatorPath)
	defer subnetConfigFile.Close()
	if err != nil {
		return err
	}
	subnetJson := make([]byte, 2000)
	n, err := subnetConfigFile.Read(subnetJson)
	if err != nil {
		return err
	}

	err = json.Unmarshal(subnetJson[:n], ipam.Subnets)
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
		return err
	}

	log.Infof("$ open %s = %s...%s", ipam.SubnetAllocatorPath, subnetJson[:23], subnetJson[n-5:n])
	return nil
}

func (ipam *IPAM) dump() error {
	ipamConfigFileDir, _ := path.Split(ipam.SubnetAllocatorPath)
	if _, err := os.Stat(ipamConfigFileDir); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(ipamConfigFileDir, 0644)
		} else {
			return err
		}
	}
	subnetConfigFile, err := os.OpenFile(ipam.SubnetAllocatorPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	defer subnetConfigFile.Close()
	if err != nil {
		return err
	}

	ipamConfigJson, err := json.Marshal(ipam.Subnets)
	if err != nil {
		return err
	}

	_, err = subnetConfigFile.Write(ipamConfigJson)
	if err != nil {
		return err
	}

	return nil
}

func (ipam *IPAM) Allocate(subnet *net.IPNet) (ip net.IP, err error) {
	// String that used to save ip allocation info for each subnet
	ipam.Subnets = &map[string]string{}

	// Load network segment info from file `ipamDefaultAllocatorPath`
	// .../ipam/subnet.json = {"192.168.0.0/24":"1111111...0000"} > ipam.Subnets
	// 192.168.0.0/24 = IP: 192.168.0.0  Mask: 255.255.255.0 (= /24)
	err = ipam.load()
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
	}

	_, subnet, _ = net.ParseCIDR(subnet.String())

	// Size() returns the number of leading ones and total bits in the mask
	// If the mask is not in the canonical form--ones followed by zeros
	// --then Size returns 0, 0.
	// For 192.168.0.0/24, one = 24, size = 32
	// Available ip addrs: 192.168.0.0~192.168.0.254
	// 192.168.0.255 is researved for broadcasting
	one, size := subnet.Mask.Size()

	// If the specified subnet not exists in the json config file
	if _, exist := (*ipam.Subnets)[subnet.String()]; !exist {
		// Use a 0 to represent each host IP in the subnet
		// 192.168.0.0/24 corresponds to 1<<(32-24) = 256 available hosts
		(*ipam.Subnets)[subnet.String()] = strings.Repeat("0", 1<<uint8(size-one))
	}

	for c := range (*ipam.Subnets)[subnet.String()] {
		// Find the entry marks 0 which is available to allocate IP
		if (*ipam.Subnets)[subnet.String()][c] == '0' {
			// Change 0 to 1, marking the host ip is occupied
			ipalloc := []byte((*ipam.Subnets)[subnet.String()])
			ipalloc[c] = '1'
			(*ipam.Subnets)[subnet.String()] = string(ipalloc)
			// The IP here is the initial IP, for subnet 192.168.0.0/24,
			// ip = 192.168.0.0
			ip = subnet.IP
			// Add offset to the initial ip address
			//      192          168          0            0
			//       +            +           +            +
			// uint8(c>>24) uint8(c>>16) uint8(c>>8)  uint8(c>>0)
			for t := uint(4); t > 0; t -= 1 {
				[]byte(ip)[4-t] += uint8(c >> ((t - 1) * 8))
			}
			// Because the ip is allocated from 1, here we add 1 to the
			// last 8 bits (ip[3])
			ip[3] += 1
			break
		}
	}

	// Dump subnet ip allocation setting to the config file
	ipam.dump()
	return
}

func (ipam *IPAM) Release(subnet *net.IPNet, ipaddr *net.IP) error {
	ipam.Subnets = &map[string]string{}

	_, subnet, _ = net.ParseCIDR(subnet.String())

	err := ipam.load()
	if err != nil {
		log.Errorf("Error dump allocation info, %v", err)
	}

	// Reverse calculation of IPAM.Allocate
	c := 0
	releaseIP := ipaddr.To4()
	releaseIP[3] -= 1
	for t := uint(4); t > 0; t -= 1 {
		c += int(releaseIP[t-1]-subnet.IP[t-1]) << ((4 - t) * 8)
	}

	// Reset corresponding bit to 0
	ipalloc := []byte((*ipam.Subnets)[subnet.String()])
	ipalloc[c] = '0'
	(*ipam.Subnets)[subnet.String()] = string(ipalloc)

	// Dump subnet ip allocation setting to the config file
	ipam.dump()
	return nil
}
