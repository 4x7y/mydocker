package network

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"net"
	//"os"
	"../container"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
)

var (
	defaultNetworkPath = "/var/run/mydocker/network/network/"
	drivers            = map[string]NetworkDriver{}
	networks           = map[string]*Network{}
)

type Endpoint struct {
	ID          string           `json:"id"`
	Device      netlink.Veth     `json:"dev"`
	IPAddress   net.IP           `json:"ip"`
	MacAddress  net.HardwareAddr `json:"mac"`
	Network     *Network
	PortMapping []string
}

// A network is a set of containers that can communicate with
// each other. Network contains network configurations including
// network ipnet address, network driver, etc.
type Network struct {
	Name    string
	IpRange *net.IPNet
	Driver  string
}

// Network driver is a component of a Network
// Network drivers have different strategies to create, connect,
// disconnect and delete a network.
// When create a network, we can specify a network driver to
// define the network behavior.
// Here we define the network driver interface:
type NetworkDriver interface {
	Name() string
	Create(subnet string, name string) (*Network, error)
	Delete(network Network) error
	Connect(network *Network, endpoint *Endpoint) error
	Disconnect(network Network, endpoint *Endpoint) error
}

func (nw *Network) dump(dumpPath string) error {
	if _, err := os.Stat(dumpPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(dumpPath, 0644)
			log.Infof("$ mkdir -p %s -m 0644", dumpPath)
		} else {
			return err
		}
	}

	nwPath := path.Join(dumpPath, nw.Name)
	nwFile, err := os.OpenFile(nwPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Errorf("error：", err)
		return err
	}
	defer nwFile.Close()

	nwJson, err := json.Marshal(nw)
	if err != nil {
		log.Errorf("error：", err)
		return err
	}

	log.Infof("$ echo %v > %s", *nw, dumpPath)
	_, err = nwFile.Write(nwJson)
	if err != nil {
		log.Errorf("error：", err)
		return err
	}
	return nil
}

func (nw *Network) remove(dumpPath string) error {
	if _, err := os.Stat(path.Join(dumpPath, nw.Name)); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	} else {
		log.Infof("$ rm -rf %s", path.Join(dumpPath, nw.Name))
		return os.Remove(path.Join(dumpPath, nw.Name))
	}
}

func (nw *Network) load(dumpPath string) error {
	nwConfigFile, err := os.Open(dumpPath)
	defer nwConfigFile.Close()
	if err != nil {
		return err
	}
	nwJson := make([]byte, 2000)
	n, err := nwConfigFile.Read(nwJson)
	if err != nil {
		return err
	}

	log.Infof("$ open %s = %s ...", dumpPath, nwJson[:20])
	err = json.Unmarshal(nwJson[:n], nw)
	if err != nil {
		log.Errorf("Error load nw info", err)
		return err
	}
	return nil
}

func LoadExistNetwork() error {
	var bridgeDriver = BridgeNetworkDriver{}
	drivers[bridgeDriver.Name()] = &bridgeDriver
	log.Infof("Load \"bridge\" network driver as default.")

	if _, err := os.Stat(defaultNetworkPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(defaultNetworkPath, 0644)
			log.Infof("mkdir %s -m 0644", defaultNetworkPath)
		} else {
			return err
		}
	}

	log.Infof("Walk through default network path %s.", defaultNetworkPath)
	filepath.Walk(defaultNetworkPath, func(nwPath string, info os.FileInfo, err error) error {
		// HasSuffix tests whether the string nwPath ends with "/"
		if strings.HasSuffix(nwPath, "/") {
			return nil
		}
		_, nwName := path.Split(nwPath) // Separating into a dir and file name
		nw := &Network{
			Name: nwName, // same to the filename
		}

		log.Infof("Load network: %s", nwName)
		if err := nw.load(nwPath); err != nil {
			log.Errorf("error load network: %s", err)
		}

		networks[nwName] = nw
		return nil
	})

	//log.Infof("networks: %v", networks)

	return nil
}

func CreateNetwork(driver, subnet, name string) error {
	// "192.0.2.1/24" -> (192.0.2.1, 192.0.2.0/24).
	_, cidr, _ := net.ParseCIDR(subnet)
	ip, err := ipAllocator.Allocate(cidr)
	if err != nil {
		return err
	}
	cidr.IP = ip

	// cidr is-a IPNet {
	//	 IP   IP      // network number
	//	 Mask IPMask  // network mask
	// }
	//
	// For example, cidr = 192.0.2.1/24
	log.Infof("Allocated IP: %s", cidr.IP)
	nw, err := drivers[driver].Create(cidr.String(), name)
	if err != nil {
		return err
	}

	return nw.dump(defaultNetworkPath)
}

func ListNetwork() {
	// NewWriter(output, minwidth, tabwidth, padding, padchar, flags)
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "NAME\tIpRange\tDriver\n")
	for _, nw := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			nw.Name,
			nw.IpRange.String(),
			nw.Driver,
		)
	}
	if err := w.Flush(); err != nil {
		log.Errorf("Flush error %v", err)
		return
	}
}

func DeleteNetwork(networkName string) error {
	nw, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}

	if err := ipAllocator.Release(nw.IpRange, &nw.IpRange.IP); err != nil {
		return fmt.Errorf("Error Remove Network gateway ip: %s", err)
	}

	if err := drivers[nw.Driver].Delete(*nw); err != nil {
		return fmt.Errorf("Error Remove Network DriverError: %s", err)
	}

	return nw.remove(defaultNetworkPath)
}

func enterContainerNetns(enLink *netlink.Link, cinfo *container.ContainerInfo) func() {
	log.Infof("Enter container %s network namespace ...", cinfo.Pid)

	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", cinfo.Pid), os.O_RDONLY, 0)
	if err != nil {
		log.Errorf("error get container net namespace, %v", err)
	}
	log.Infof("$ open %s", fmt.Sprintf("/proc/%s/ns/net", cinfo.Pid))

	// Network namespace file descriptor of container process
	nsFD := f.Fd()
	runtime.LockOSThread()

	// 修改veth peer 另外一端移到容器的namespace中
	// Puts the device into a new network namespace. The `fd` must be an open file
	// descriptor to a network namespace.
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		log.Errorf("error set link netns , %v", err)
	}
	log.Infof("// Puts the device %v into container network namespace", (*enLink).Attrs().Name)
	log.Infof("$ ip link set dev %v netns %d", (*enLink).Attrs().Name, int(nsFD))

	// 获取当前的网络namespace
	// Gets a handle to the current threads network namespace. (NsHandle)
	origns, err := netns.Get()
	if err != nil {
		log.Errorf("error get current netns, %v", err)
	}

	// 设置当前进程到新的网络namespace，并在函数执行完成之后再恢复到之前的namespace
	// Sets the current network namespace to the namespace represented by NsHandle
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		log.Errorf("error set netns, %v", err)
	}

	return func() {
		// Teardown
		netns.Set(origns)
		origns.Close()
		runtime.UnlockOSThread()
		f.Close()
	}
}

func configEndpointIpAddressAndRoute(ep *Endpoint, cinfo *container.ContainerInfo) error {
	log.Infof("Config endpoint ip address and route ...")

	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return fmt.Errorf("fail config endpoint: %v", err)
	}

	// Enter container network namespace and put veth peer device to it
	defer enterContainerNetns(&peerLink, cinfo)()

	// Get container ip address and ip segment, so as to config the container interface
	// For example, if contianer ip is 192.168.1.2, the subnet is 192.168.1.0/24
	// then the resulting ip address is 192.168.1.2/24
	interfaceIP := *ep.Network.IpRange
	interfaceIP.IP = ep.IPAddress

	// Setup device ip inside the container namespace
	if err = setInterfaceIP(ep.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%s", ep.Network, err)
	}

	// Start up that veth endpoint
	if err = setInterfaceUP(ep.Device.PeerName); err != nil {
		return err
	}

	// In the net namespace, local address 127.0.0.0 device lo is turned off as default
	// Here we start it up so that the container can access itself
	if err = setInterfaceUP("lo"); err != nil {
		return err
	}

	// Let all outgoing requests accessed through veth endpoint
	// 0.0.0.0/0 represents all ip address
	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")
	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        ep.Network.IpRange.IP,
		Dst:       cidr,
	}
	if err = netlink.RouteAdd(defaultRoute); err != nil {
		return err
	}
	log.Infof("$ ip route add -net 0.0.0.0/0 gw %v dev %v", defaultRoute.Gw, defaultRoute.LinkIndex)

	return nil
}

func configPortMapping(ep *Endpoint, cinfo *container.ContainerInfo) error {
	for _, pm := range ep.PortMapping {
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			log.Errorf("port mapping format error, %v", pm)
			continue
		}
		iptablesCmd := fmt.Sprintf("-t nat -A PREROUTING -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		log.Infof("$ iptables %s", iptablesCmd)

		//err := cmd.Run()
		output, err := cmd.Output()
		if err != nil {
			log.Errorf("iptables Output, %v", output)
			continue
		}
	}
	return nil
}

func Connect(networkName string, cinfo *container.ContainerInfo) error {
	network, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("No Such Network: %s", networkName)
	}

	// Allocate container IP address
	ip, err := ipAllocator.Allocate(network.IpRange)
	if err != nil {
		return err
	}
	log.Infof("Allocated IP: %s", ip)

	// Create network endpoint
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", cinfo.Id, networkName),
		IPAddress:   ip,
		Network:     network,
		PortMapping: cinfo.PortMapping,
	}
	// Invoke network driver connect and setup endpoint
	if err = drivers[network.Driver].Connect(network, ep); err != nil {
		return err
	}
	// 到容器的namespace配置容器网络设备IP地址
	if err = configEndpointIpAddressAndRoute(ep, cinfo); err != nil {
		return err
	}

	return configPortMapping(ep, cinfo)
}

func Disconnect(networkName string, cinfo *container.ContainerInfo) error {
	return nil
}
