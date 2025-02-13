package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type installerDownloadCmd struct {
	aerospikeVersionSelectorCmd
	IsArm bool    `long:"arm" description:"indicate installing on an arm instance"`
	Help  helpCmd `command:"help" subcommands-optional:"true" description:"Print help"`
}

func (c *installerDownloadCmd) Execute(args []string) error {
	if earlyProcessV2(args, false) {
		return nil
	}
	_, err := c.runDownload(args)
	return err
}

func (c *installerDownloadCmd) runDownload(args []string) (string, error) {
	log.Print("Running installer.download")
	if err := chDir(string(c.ChDir)); err != nil {
		return "", logFatal("ChDir failed: %s", err)
	}
	var url string
	var err error
	bv := &backendVersion{c.DistroName.String(), c.DistroVersion.String(), c.AerospikeVersion.String(), c.IsArm}
	if strings.HasPrefix(c.AerospikeVersion.String(), "latest") || strings.HasSuffix(c.AerospikeVersion.String(), "*") || strings.HasPrefix(c.DistroVersion.String(), "latest") {
		url, err = aerospikeGetUrl(bv, c.Username, c.Password)
		if err != nil {
			return "", fmt.Errorf("aerospike Version not found: %s", err)
		}
		c.AerospikeVersion = TypeAerospikeVersion(bv.aerospikeVersion)
		c.DistroName = TypeDistro(bv.distroName)
		c.DistroVersion = TypeDistroVersion(bv.distroVersion)
	}

	log.Printf("Distro = %s:%s ; AerospikeVersion = %s", c.DistroName, c.DistroVersion, c.AerospikeVersion)
	verNoSuffix := strings.TrimSuffix(c.AerospikeVersion.String(), "c")
	verNoSuffix = strings.TrimSuffix(verNoSuffix, "f")
	// check if template exists
	if url == "" {
		url, err = aerospikeGetUrl(bv, c.Username, c.Password)
		if err != nil {
			return "", fmt.Errorf("aerospike Version URL not found: %s", err)
		}
		c.AerospikeVersion = TypeAerospikeVersion(bv.aerospikeVersion)
		c.DistroName = TypeDistro(bv.distroName)
		c.DistroVersion = TypeDistroVersion(bv.distroVersion)
	}

	var edition string
	if strings.HasSuffix(c.AerospikeVersion.String(), "c") {
		edition = "aerospike-server-community"
	} else if strings.HasSuffix(c.AerospikeVersion.String(), "f") {
		edition = "aerospike-server-federal"
	} else {
		edition = "aerospike-server-enterprise"
	}
	archString := ".x86_64"
	if bv.isArm {
		archString = ".arm64"
	}
	fn := edition + "-" + verNoSuffix + "-" + c.DistroName.String() + c.DistroVersion.String() + archString + ".tgz"
	// download file if not exists
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		log.Println("Downloading installer")
		err = downloadFile(url, fn, c.Username, c.Password)
		if err != nil {
			return "", err
		}
	}
	log.Print("Done")
	return fn, nil
}
