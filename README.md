# revpn - Sonicwall with easy route control

This is a classic solving my own problem project - I use Sonicwall at work but
I don't want all the routes it pushes, particularly I don't want to have the
default route over the VPN because it is slower than my home internet connection.

## Installing

```bash
$ go get github.com/dooferlad/revpn
```

## Usage

First write a `.revpn.yaml` in your home directory:

```yaml
netExtender: /usr/sbin/netExtender
vpnuser: <your username>
password: <your password>
domain: <domain you are connecting into>

vpn_host: <ip address of VPN server>:<port>

# Machines that you know by IP address that you need to be on the VPN to access
routed_hosts:
  - server.on.vpn
  - server1.on.vpn

# For machines without a DNS entry, use this list...
routed_addresses:
  - 1.1.1.1
  - 8.8.8.8
```

```bash
$ revpn
```

## Notes
I use this on Ubuntu with Sonicwall NetExtender. If you would like to adapt it
to run on other operating systems and with other VPN clients then I will
welcome your PR.

I currently don't have any tests - honestly, it is trivial and I have been using
it for a while so I don't anticipate any problems. If problems turn up, tests
will be added around fixes :-)
