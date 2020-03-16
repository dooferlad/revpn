package cmd

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/dooferlad/jat/cmd"
)

func routedIPs() ([]string, error) {
	var ips []string

	for _, host := range viper.GetStringSlice("routed_hosts") {

		i, err := net.LookupIP(host)
		if err != nil {
			return ips, errors.Wrap(err, "IP lookup failed")
		}
		for _, ip := range i {
			ips = append(ips, ip.String())
		}
	}

	for _, addr := range viper.GetStringSlice("routed_addresses") {
		ips = append(ips, addr)
	}

	return ips, nil
}

func start() (*exec.Cmd, error) {
	command := exec.Command(
		viper.GetString("netExtender"),
		"-u", viper.GetString("vpnuser"),
		"-p", viper.GetString("password"),
		"-d", viper.GetString("domain"),
		"--dns-only-local",
		viper.GetString("vpn_host"))

	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "stdout pipe error")
	}
	if err := command.Start(); err != nil {
		return nil, errors.Wrap(err, "netExtender command failed")
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
		if strings.HasPrefix(scanner.Text(), "NetExtender connected successfully") {
			return command, nil
		}
	}

	return nil, errors.New("unable to connect VPN")
}

func gateway() string {
	command := exec.Command("ip", "route")
	stdout, err := command.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := command.Start(); err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "default via") {
			return strings.Split(scanner.Text(), " ")[2]
		}
	}

	return ""
}

func ppp0Routes() []string {
	command := exec.Command("ip", "route")
	stdout, err := command.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := command.Start(); err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(stdout)
	var routes []string
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " \n")
		if strings.Contains(line, "dev ppp0") {
			routes = append(routes, line)
		}
	}

	return routes
}

func reroute() error {
	time.Sleep(time.Second)
	gw := gateway()
	log.Info("Got gateway ", gw)

	log.Infof("Removing new default route %v", gw)
	if err := cmd.Sudo("route", "del", "default", "gw", gw); err != nil {
		return err
	}

	// Delete routes that ppp0 currently has
	for _, route := range ppp0Routes() {
		log.Debug(route)
		log.Info("Delete ", route)
		args := []string{"route", "del"}
		args = append(args, strings.Split(route, " ")...)
		if err := cmd.Sudo("ip", args...); err != nil {
			return err
		}
	}

	toRoute, err := routedIPs()
	if err != nil {
		return err
	}

	for _, ip := range toRoute {
		log.Infof("Routing %v via VPN", ip)
		if err := cmd.Sudo("route", "add", ip, "gw", gw); err != nil {
			return err
		}
	}

	return nil
}

func connect() error {
	log.Info("Connecting")
	cmd.Sudo("echo", "sudo please")

	vpn, err := start()
	if err != nil {
		return err
	}
	defer vpn.Process.Kill()

	log.Info("Connected")
	if !noReroute {
		if err := reroute(); err != nil {
			return err
		}
	}

	// Now print the results of our efforts!
	if err := cmd.Shell("route", "-n"); err != nil {
		return err
	}

	if err := vpn.Wait(); err != nil {
		return err
	}

	return nil
}
