package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/dooferlad/jat/cmd"
)

func routedIPs() ([]string, error) {
	var ips []string

	for _, host := range viper.GetStringSlice("routed_hosts") {

		i, err := net.LookupIP(host)
		if err != nil {
			return ips, err
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
	var command *exec.Cmd
	if viper.GetString("vpnScript") != "" {
		command = exec.Command(viper.GetString("vpnScript"))
	} else {
		command = exec.Command(
			viper.GetString("netExtender"),
			"-u", viper.GetString("vpnuser"),
			"-p", viper.GetString("password"),
			"-d", viper.GetString("domain"),
			"--dns-only-local",
			viper.GetString("vpn_host"))
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmdIn, err := command.StdinPipe()
	if err != nil {
		log.Fatal("Unable to open stdin to VPN command")
	}
	if err := command.Start(); err != nil {
		return nil, err
	}

	stdin := bufio.NewScanner(os.Stdin)

	scanner := bufio.NewScanner(stdout)
	lines := make(chan string, 2)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	var line string

	for {
		select {
		case line = <-lines:
			fmt.Println(line)
			if strings.HasPrefix(line, "Do you want to proceed? (Y:Yes, N:No, V:View Certificate)") {
				stdin.Scan()
				input := stdin.Text()
				fmt.Println(input)
				if _, err := io.WriteString(cmdIn, input); err != nil {
					return nil, errors.Wrap(err, "Unable to write to command")
				}
			}

			// Netextender has connected
			if strings.HasPrefix(line, "NetExtender connected successfully") {
				return command, nil
			}

			// openconnect has connected
			if strings.HasPrefix(line, "Connected as") {
				return command, nil
			}
		}
	}
}

func gateway() (string, bool, string, error) {
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
		fields := strings.Fields(scanner.Text())
		fmt.Printf("%#v\n", fields)
		if fields[0] == "default" {
			if fields[1] == "via" {
				return fields[2], false, fields[4], nil
			}

			if fields[1] == "dev" {
				return fields[2], true, fields[2], nil
			}
		}
	}

	return "", false, "", fmt.Errorf("unable to find default route")
}

func deviceRoutes(device string) []string {
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
	searchString := "dev " + device

	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " \n")
		log.Info(line)
		if strings.Contains(line, searchString) {
			routes = append(routes, line)
		}
	}

	return routes
}

func reroute() error {
	time.Sleep(time.Second)
	gw, gwIsDev, device, err := gateway()
	if err != nil {
		return err
	}
	log.Info("Got gateway ", gw, "via", device)

	var arg string
	if gwIsDev {
		arg = "dev"
	} else {
		arg = "gw"
	}

	log.Infof("Removing new default route %v", gw)
	if err := cmd.Sudo("route", "del", "default", arg, gw); err != nil {
		return err
	}

	// Delete routes that the default gateway
	for _, route := range deviceRoutes(device) {
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
		if err := cmd.Sudo("route", "add", ip, arg, gw); err != nil {
			return err
		}
	}

	return nil
}

func connect() error {
	log.Info("Connecting")
	cmd.Sudo("echo", "sudo please")

	var vpn *exec.Cmd
	var err error

	if !noVPN {
		vpn, err = start()
		if err != nil {
			return err
		}
		defer vpn.Process.Kill()
	}

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

	if !noVPN {
		if err := vpn.Wait(); err != nil {
			return err
		}
	}

	return nil
}
