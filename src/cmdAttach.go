package main

import "os"

type attachCmd struct {
	Shell  attachShellCmd  `command:"shell" subcommands-optional:"true" description:"Attach to shell"`
	Client attachClientCmd `command:"client" subcommands-optional:"true" description:"Attach to client machine shell"`
	Aql    attachAqlCmd    `command:"aql" subcommands-optional:"true" description:"Run aql on node"`
	Asadm  attachAsadmCmd  `command:"asadm" subcommands-optional:"true" description:"Run asadm on node"`
	Asinfo attachAsinfoCmd `command:"asinfo" subcommands-optional:"true" description:"Run asinfo on node"`
	AGI    agiAttachCmd    `command:"agi" subcommands-optional:"true" description:"Attach to an AGI node"`
	Help   attachCmdHelp   `command:"help" subcommands-optional:"true" description:"Print help"`
}

func (c *attachCmd) Execute(args []string) error {
	c.Help.Execute(args)
	os.Exit(1)
	return nil
}
