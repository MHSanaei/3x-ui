# TUN network layer 3 input support

TUN interface support bridges the gap between network layer 3 and layer 7, introducing raw network input.

This functionality is targeted to assist applications/end devices that don't have proxy support, or can't run external applications (like Smart TV's). Making it possible to run Xray proxy right on network edge devices (routers) with support to route raw network traffic. \
Primary targets are Linux based router devices. Like most popular OpenWRT option. \
Support for Windows, macOS, Android and iOS is also implemented (see below).

## PLEASE READ FOLLOWING CAREFULLY

If you are not sure what this is and do you need it or not - you don't. \
This functionality is intended to be configured by network professionals, who understand the deployment case and scenarios. \
Plainly enabling it in the config probably will result nothing, or lock your router up in infinite network loop.

## DETAILS

Current implementation does not contain options to configure network level addresses, routing or rules.
Enabling the feature will result only tun interface up, and that's it. \
This is explicit decision, significantly simplifying implementation, and allowing any number of custom configurations, consumers could come up with. Network interface is OS level entity, and OS is what should manage it. \
Working configuration, is tun enabled in Xray config with specific name (e.g. xray0), and OS level configuration to manage "xray0" interface, applying routing and rules on interface up.
This way consistency of system level routing and rules is ensured from single place of responsibility - the OS itself. \
Examples of how to achieve this on a simple Linux system (Ubuntu with systemd-networkd) can be found at the end of this README.

Due to this inbound not actually being a proxy, the configuration ignore required listen and port options, and never listen on any port. \
Here is simple Xray config snippet to enable the inbound:
```
{
  "inbounds": [
    {
      "port": 0,
      "protocol": "tun",
      "settings": {
        "name": "xray0",
        "MTU": 1492
      }
    }
  ],
```

## SUPPORTED FEATURES

- IPv4 and IPv6
- TCP and UDP

## LIMITATION

- No ICMP support
- Connections are established to any host, as connection success is only a mark of successful accepting packet for proxying. Hosts that are not accepting connections or don't even exists, will look like they opened a connection (SYN-ACK), and never send back a single byte, closing connection (RST) after some time. This is the side effect of the whole process actually being a proxy, and not real network layer 3 vpn

## CONSIDERATIONS

This feature being network level interface that require raw routing, bring some ambiguities that need to be taken in account. \
Xray-core itself is connecting to its uplinks on a network level, therefore, it's really simple to lock network up in an infinite loop, when trying to pass "everything through Xray". \
You can't just route 0.0.0.0/0 through xray0 interface, as that will result Xray-core itself try to reach its uplink through xray0 interface, resulting infinite network loop.
There are several ways to address this:

- Approach 1: \
  Add precise static route to Xray upstreams, having them always routed through static internet gateway.
  E.g. when 123.123.123.123 is the Xray VLESS uplink, this network configuration will work just fine:
  ```
  ip route add 123.123.123.123/32 via <provider internet gateway ip>
  ip route add 0.0.0.0/0 dev xray0
  ```
  This has disadvantages, - a lot of conditions must be kept static: internet gateway address, xray uplink ip address, and so on.
- Approach 1-b: \
  Route only specific networks through Xray, keeping the default gateway unchanged.
  This can be done in many different ways, using ip sets, routing daemons like BGP peers, etc... All you need to do is to route the paths through xray0 dev.
  The disadvantage in this case is smaller, - you need to make sure the uplink will not become part of those sets and that's it. Can easily be done with route metric priorities.
- Approach 2: \
  Separate main route table and Xray route table with default gateways pointing to different destinations.
  This way you can achieve full protection of hosts behind the router, keeping router configuration as flexible as desired. \
  There are two ways to do that: \
  Either configure xray0 interface to appear and operate as default gateway in a separate route table, e.g. 1001. Then mark and route protected traffic by ip rules to that table. \
  It's a simplest way to make a "non-damaging" configuration, when the only thing you need to do to enable/disable proxying is to flip the ip rules off. Which is also a disadvantage of itself - if by accident ip rules will get disabled, the traffic will spill out of the internet interface unprotected. \
  Or, other way around, move default routing to a separate route table, so that all usual routing information is set in e.g. route table 1000,
  and Xray interface operate in the main route table. This will allow proper flexibility, but you need to ensure traffic from the Xray process, running on the router, is marked to get routed through table 1000. This again can be achieved in same ways, using ip rules and iptable rules combinations. \
  Big advantage of that, is that traffic of the route itself is going to be wrapped into the proxy, including DNS queries, without any additional effort. Although, the disadvantage of that is, that in any case proxying stops (uplink dies, Xray hangs, encryption start to flap), it will result complete internet inaccessibility. \
  Any approach is applicable, and if you know what you are doing (which is expected, if you read until here) you do understand which one you want and can manage. \

### Important:

TUN is network level entity, therefore communication through it, is always ip to ip, there are no host names. \
Therefore, DNS resolution will always happen before traffic even enter the interface (it will be separate ip-to-ip packets/connections to resolve hostnames). \
You always need to consider that DNS queries in any configuration you chose, most likely, will originate from the router itself (hosts behind the router access router DNS, router DNS fire queries to the outside).
Without proper addressing that, DNS queries will expose actual destinations/websites accessed through the router. \
To address that you can as ignore (not use) DNS of the router (just delegate some public DNS in DHCP configuration to your devices), or make sure routing rules are configured the way, DNS resolution of the router itself runs through Xray interface/routing table.

You also need to remember that local traffic of the router (e.g. DNS, firmware updates, etc.), is subject of firewall rules as outcoming/incoming traffic (not forward).
If you have restrictive firewall, you need to allow input/output traffic through xray0 interface, for it to properly dispatch and reach the OS back.

Additionally, working with two route tables is not taken lightly by Linux, and sometimes make it panic about "martian packets", which it calls the packets arriving through interfaces it does not expect they could arrive from. \
It was just a warning until recent kernel versions, but now traffic is often dropped. \
In simple case this can be just disabled with
```
/usr/sbin/sysctl -w net.ipv4.conf.all.rp_filter=0
```
But proper approach should be defining your route tables fully and consistently, adding all routes corresponding to traffic that flow through them.

## EXAMPLES

systemd-networkd \
configuration file you can place in /etc/systemd/networkd as 90-xray0.network
which will configure xray0 interface and routing using route table 1001, when the interface will appear in the system (Xray starts). And deconfigure when disappears.
```
[Match]
Name = xray0

[Network]
KeepConfiguration = yes

[Link]
ActivationPolicy = manual
RequiredForOnline = no

[Route]
Table = 1001
Destination = 0.0.0.0/0

[RoutingPolicyRule]
From = 192.168.0.0/24
Table = 1001
```
RoutingPolicyRule will add the record into ip rules, that will funnel all traffic from 192.168.0.0/24 through the table 1001 \
Please note that for ideal configuration of the routing you will also need to add the route to 192.168.0.0/24 to the route table 1001.
You can do that e.g. in the file, describing your adapter serving local network (e.g. 10-br0.network), additionally to its native properties like:
```
[Match]
Name = br0
Type = bridge

[Network]
...skip...
Address = 192.168.0.1/24
...skip...

[Route]
Table = 1001
Destination = 192.168.0.0/24
PreferredSource = 192.168.0.1
Scope = link
```
All in all systemd-networkd and its derivatives (like netplan or NetworkManager) provide all means to configure your networking, according to your wish, that will ensure network consistency of xray0 interface coming up and down, relative to other network configuration like internet interfaces, nat rules and so on.

## WINDOWS SUPPORT

Windows version of the same functionality is implemented through Wintun library. \
To make it start, wintun.dll specific for your Windows/arch must be present next to Xray.exe binary.

After the start network adapter with the name you chose in the config will be created in the system, and exist while Xray is running.

You can give the adapter ip address manually, you can live Windows to give it autogenerated ip address (which take few seconds), it doesn't matter, the traffic going _through_ the interface will be forwarded into the app for proxying. \
Minimal configuration that will work for local machine is routing passing the traffic on-link through the interface.
You will need the interface id for that, unfortunately it is going to change with every Xray start due to implementation ambiguity between Xray and wintun driver.
You can find the interface id with the command
```
route print
```
it will be in the list of interfaces on the top of the output
```
===========================================================================
Interface List
  8...cc cc cc cc cc cc ......Realtek PCIe GbE Family Controller
 47...........................Xray Tunnel
  1...........................Software Loopback Interface 1
===========================================================================
```
In this case the interface id is "47". \
Then you can add on-link route through the adapter with (example) command
```
route add 1.1.1.1 mask 255.0.0.0 0.0.0.0 if 47
```
Note on ipv6 support. \
Despite Windows also giving the adapter autoconfigured ipv6 address, the ipv6 is not possible until the interface has any _routable_ ipv6 address (given link-local address will not accept traffic from external addresses). \
So everything applicable for ipv4 above also works for ipv6, you only need to give the interface some address manually, e.g. anything private like fc00::a:b:c:d/64 will do just fine

## MAC OS X SUPPORT

Darwin (Mac OS X) support of the same functionality is implemented through utun (userspace tunnel).

Interface name in the configuration must comply to the scheme "utunN", where N is some number. \
Most running OS'es create some amount of utun interfaces in advance for own needs. Please either check the interfaces you already have occupied by issuing following command:
```
ifconfig
```
Produced list will have all system interfaces listed, from which you will see how many "utun" ones already exists.
It's not required to select next available number, e.g. if you have utun1-utun7 interfaces, it's not required to have "utun8" in the config. You can choose any available name, even utun20, to get surely available interface number.

To attach routing to the interface, route command like following can be executed:
```
sudo route add -net 1.1.1.0/24 -iface utun10
```
```
sudo route add -inet6 -host 2606:4700:4700::1111 -iface utun10
sudo route add -inet6 -host 2606:4700:4700::1001 -iface utun10
```
Important to remember that everything written above about Linux routing concept, also apply to Mac OS X. If you simply route default route through utun interface, that will result network loop and immediate network failure.

## ANDROID SUPPORT

Android uses the VpnService API which provides a TUN file descriptor to the application.

Obtain the fd from VpnService:
```kotlin
val tunFd = vpnInterface.fd
```

Set the environment variable `xray.tun.fd` (or `XRAY_TUN_FD`) to the fd number before starting Xray. This can be done from Kotlin/Java or by exposing a Go function via gomobile bindings.

Build using gomobile for Android library integration:
```
gomobile bind -target=android
```

## iOS SUPPORT

iOS uses the same utun packet format as macOS, but the file descriptor is provided by NetworkExtension.

Obtain the fd from NetworkExtension:
```swift
var buf = [CChar](repeating: 0, count: Int(IFNAMSIZ))
let utunPrefix = "utun".utf8CString.dropLast()
let tunFd = ((0 ... 1024).first { (_ fd: Int32) -> Bool in var len = socklen_t(buf.count)
    return getsockopt(fd, 2, 2, &buf, &len) == 0 && buf.starts(with: utunPrefix)
}!
```

Set the environment variable `xray.tun.fd` (or `XRAY_TUN_FD`) to the fd number before starting Xray. This can be done from Swift/Objective-C or by exposing a Go function via gomobile bindings.

Build using gomobile for iOS framework integration:
```
gomobile bind -target=ios
```