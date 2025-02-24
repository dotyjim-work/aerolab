package main

import (
	"fmt"
	"log"
	"strings"
)

type clientConfigureAMSCmd struct {
	ClientName      TypeClientName  `short:"n" long:"group-name" description:"Client group name" default:"client"`
	Machines        TypeMachines    `short:"l" long:"machines" description:"Comma separated list of machines, empty=all" default:""`
	ConnectClusters TypeClusterName `short:"s" long:"clusters" default:"mydc" description:"comma-separated list of clusters to configure as source for this AMS"`
	Help            helpCmd         `command:"help" subcommands-optional:"true" description:"Print help"`
}

func (c *clientConfigureAMSCmd) Execute(args []string) error {
	if earlyProcess(args) {
		return nil
	}
	log.Print("Running client.configure.ams")
	a.opts.Attach.Client.ClientName = c.ClientName
	if c.Machines == "" {
		c.Machines = "ALL"
	}
	a.opts.Attach.Client.Machine = c.Machines
	nodeList, err := c.checkClustersExist(c.ConnectClusters.String())
	if err != nil {
		return err
	}
	b.WorkOnClients()
	allnodes := []string{}
	allnodeExp := []string{}
	for _, nodes := range nodeList {
		for _, node := range nodes {
			allnodes = append(allnodes, node+":9145")
			allnodeExp = append(allnodeExp, node+":9100")
		}
	}
	ips := "'" + strings.Join(allnodes, "','") + "'"
	nips := "'" + strings.Join(allnodeExp, "','") + "'"
	defer backendRestoreTerminal()
	err = a.opts.Attach.Client.run([]string{"sed", "-i.bakX", "-E", "s/.*TODO_ASD_TARGETS/      - targets: [" + ips + "] #TODO_ASD_TARGETS/g", "/etc/prometheus/prometheus.yml"})
	if err != nil {
		return fmt.Errorf("failed to configure prometheus (sed): %s", err)
	}
	err = a.opts.Attach.Client.run([]string{"sed", "-i.bakY", "-E", "s/.*TODO_ASDN_TARGETS/      - targets: [" + nips + "] #TODO_ASDN_TARGETS/g", "/etc/prometheus/prometheus.yml"})
	if err != nil {
		return fmt.Errorf("failed to configure prometheus (sed.1): %s", err)
	}
	// (re)start prometheus
	err = a.opts.Attach.Client.run([]string{"/bin/bash", "-c", "kill -HUP $(pidof prometheus)"})
	if err != nil {
		return fmt.Errorf("failed to restart prometheus: %s", err)
	}
	backendRestoreTerminal()
	log.Printf("To access grafana, visit the client IP on port 3000 from your browser. Do `aerolab client list` to get IPs. Username:Password is admin:admin")
	log.Print("Done")
	return nil
}
