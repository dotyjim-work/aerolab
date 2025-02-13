package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"
)

type GitHubRelease struct {
	LatestVersion  string `json:"tag_name"`
	CurrentVersion string `json:"current_version"`
}

func (a *aerolab) isLatestVersion() {
	rootDir, err := a.aerolabRootDir()
	if err != nil {
		return
	}
	versionFile := path.Join(rootDir, "version-check.json")
	v := &GitHubRelease{}
	out, err := os.ReadFile(versionFile)
	if err != nil {
		err = a.isLatestVersionQuery(v, versionFile)
		if err != nil {
			return
		}
	}
	err = json.Unmarshal(out, v)
	if err != nil || v.LatestVersion == "" || v.CurrentVersion == "" {
		err = a.isLatestVersionQuery(v, versionFile)
		if err != nil {
			return
		}
	}
	if v.CurrentVersion != vBranch {
		err = a.isLatestVersionQuery(v, versionFile)
		if err != nil {
			return
		}
	}
	if VersionCheck(v.CurrentVersion, v.LatestVersion) > 0 {
		log.Println("AEROLAB VERSION: A new version of AeroLab is available, download link: https://github.com/aerospike/aerolab/releases")
	}
}

func (a *aerolab) isLatestVersionQuery(v *GitHubRelease, versionFile string) error {
	client := &http.Client{}
	client.Timeout = 5 * time.Second
	defer client.CloseIdleConnections()
	req, err := http.NewRequest("GET", "https://api.github.com/repos/aerospike/aerolab/releases/latest", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		body, _ := io.ReadAll(response.Body)
		err = fmt.Errorf("GET 'https://api.github.com/repos/aerospike/aerolab/releases/latest': exit code (%d), message: %s", response.StatusCode, string(body))
		return err
	}
	err = json.NewDecoder(response.Body).Decode(v)
	if err != nil {
		return err
	}
	v.CurrentVersion = vBranch
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	os.WriteFile(versionFile, data, 0600)
	return nil
}
