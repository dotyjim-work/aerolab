package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/bestmethod/inslice"
)

type rosterApplyCmd struct {
	rosterShowCmd
	Roster      string `short:"r" long:"roster" description:"set this to specify customer roster; leave empty to apply observed nodes automatically" default:""`
	NoRecluster bool   `short:"c" long:"no-recluster" description:"if set, will not apply recluster command after roster-set"`
}

func (c *rosterApplyCmd) Execute(args []string) error {
	if earlyProcess(args) {
		return nil
	}

	log.Print("Running roster.apply")
	err := c.runApply(args)
	if err != nil {
		return err
	}
	log.Print("Done")
	return nil
}

func (c *rosterApplyCmd) findNodes(n int) []string {
	out, err := b.RunCommands(string(c.ClusterName), [][]string{{"asinfo", "-v", "roster:namespace=" + c.Namespace}}, []int{n})
	if err != nil {
		log.Printf("ERROR skipping node, running asinfo on node %d: %s", n, err)
		return nil
	}
	observedNodesSplit := strings.Split(strings.Trim(string(out[0]), "\t\r\n "), ":observed_nodes=")
	if len(observedNodesSplit) < 2 {
		log.Printf("ERROR skipping node, running asinfo on node %d: %s", n, out[0])
		return nil
	}
	return strings.Split(observedNodesSplit[1], ",")
}

func (c *rosterApplyCmd) findNodesParallel(node int, parallel chan int, wait *sync.WaitGroup, ob chan []string) {
	defer func() {
		<-parallel
		wait.Done()
	}()
	on := c.findNodes(node)
	ob <- on
}

func (c *rosterApplyCmd) runApply(args []string) error {
	clist, err := b.ClusterList()
	if err != nil {
		return err
	}

	if !inslice.HasString(clist, string(c.ClusterName)) {
		return errors.New("cluster does not exist")
	}

	nodes, err := b.NodeListInCluster(string(c.ClusterName))
	if err != nil {
		return err
	}

	nodesList := []int{}
	if c.Nodes == "" {
		nodesList = nodes
	} else {
		for _, nn := range strings.Split(c.Nodes.String(), ",") {
			n, err := strconv.Atoi(nn)
			if err != nil {
				return fmt.Errorf("%s is not a number: %s", nn, err)
			}
			if !inslice.HasInt(nodes, n) {
				return fmt.Errorf("node %d does not exist in cluster", n)
			}
			nodesList = append(nodesList, n)
		}
	}

	newRoster := c.Roster

	if newRoster == "" {
		foundNodes := []string{}
		if c.ParallelThreads == 1 || len(nodesList) == 1 {
			for _, n := range nodesList {
				observedNodes := c.findNodes(n)
				for _, on := range observedNodes {
					if !inslice.HasString(foundNodes, on) {
						foundNodes = append(foundNodes, on)
					}
				}
			}
		} else {
			parallel := make(chan int, c.ParallelThreads)
			wait := new(sync.WaitGroup)
			observedNodes := make(chan []string, len(nodesList))
			for _, n := range nodesList {
				parallel <- 1
				wait.Add(1)
				go c.findNodesParallel(n, parallel, wait, observedNodes)
			}
			wait.Wait()
			if len(observedNodes) > 0 {
				for _, on := range <-observedNodes {
					if !inslice.HasString(foundNodes, on) {
						foundNodes = append(foundNodes, on)
					}
				}
			}
		}
		if len(foundNodes) == 0 || inslice.HasString(foundNodes, "null") {
			return errors.New("found at least one node which thinks the observed list is 'null' or failed to find any nodes in roster")
		}
		newRoster = strings.Join(foundNodes, ",")
	}

	rosterCmd := []string{"asinfo", "-v", "roster-set:namespace=" + c.Namespace + ";nodes=" + newRoster}
	if a.opts.Config.Backend.Type != "docker" {
		rosterCmd = []string{"asinfo", "-v", "roster-set:namespace=" + c.Namespace + "\\;nodes=" + newRoster}
	}

	if c.ParallelThreads == 1 || len(nodesList) == 1 {
		c.applyRoster(nodesList, rosterCmd)
	} else {
		parallel := make(chan int, c.ParallelThreads)
		wait := new(sync.WaitGroup)
		for _, n := range nodesList {
			parallel <- 1
			wait.Add(1)
			go c.applyRosterParallel(n, rosterCmd, parallel, wait)
		}
		wait.Wait()
	}

	if c.NoRecluster {
		log.Print("Done. Roster applied, did not recluster!")
		return nil
	}

	if c.ParallelThreads == 1 || len(nodesList) == 1 {
		out, err := b.RunCommands(string(c.ClusterName), [][]string{{"asinfo", "-v", "recluster:namespace=" + c.Namespace}}, nodesList)
		if err != nil {
			outn := ""
			for _, i := range out {
				outn = outn + string(i) + "\n"
			}
			log.Printf("WARNING: could not send recluster to all the nodes: %s: %s", err, outn)
		}
	} else {
		parallel := make(chan int, c.ParallelThreads)
		wait := new(sync.WaitGroup)
		for _, n := range nodesList {
			parallel <- 1
			wait.Add(1)
			go func(n int, parallel chan int, wait *sync.WaitGroup) {
				defer func() {
					<-parallel
					wait.Done()
				}()
				out, err := b.RunCommands(string(c.ClusterName), [][]string{{"asinfo", "-v", "recluster:namespace=" + c.Namespace}}, []int{n})
				if err != nil {
					outn := ""
					for _, i := range out {
						outn = outn + string(i) + "\n"
					}
					log.Printf("WARNING: could not send recluster to all the nodes: %s: %s", err, outn)
				}
			}(n, parallel, wait)
		}
		wait.Wait()
	}
	err = c.show(args)
	if err != nil {
		return err
	}
	return nil
}

func (c *rosterApplyCmd) applyRosterParallel(node int, rosterCmd []string, parallel chan int, wait *sync.WaitGroup) {
	defer func() {
		<-parallel
		wait.Done()
	}()
	out, err := b.RunCommands(string(c.ClusterName), [][]string{rosterCmd}, []int{node})
	for _, out1 := range out {
		if strings.Contains(string(out1), "ERROR") {
			log.Print(string(out1))
		}
	}
	if err != nil {
		outn := ""
		for _, i := range out {
			outn = outn + string(i) + "\n"
		}
		log.Printf("WARNING: could not apply roster to %d: %s: %s", node, err, outn)
	}
}

func (c *rosterApplyCmd) applyRoster(nodesList []int, rosterCmd []string) {
	out, err := b.RunCommands(string(c.ClusterName), [][]string{rosterCmd}, nodesList)
	for _, out1 := range out {
		if strings.Contains(string(out1), "ERROR") {
			log.Print(string(out1))
		}
	}
	if err != nil {
		outn := ""
		for _, i := range out {
			outn = outn + string(i) + "\n"
		}
		log.Printf("WARNING: could not apply roster to all the nodes: %s: %s", err, outn)
	}
}
