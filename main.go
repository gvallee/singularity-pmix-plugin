// Copyright (c) 2020, Geoffroy Vallee <geoffroy.vallee@gmail.com>. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the URIs of this project regarding your
// rights to use or distribute this software.

package main

/*
#include <stdio.h>
#include <pmix.h>
int init_pmix() {
	int rc;
	pmix_proc_t myproc;

	fprintf(stderr, "Initializing PMIx...\n");
	rc = PMIx_Init(&myproc, NULL, 0);
	if (rc != PMIX_SUCCESS) {
		fprintf(stderr, "[ERROR] failed to initialize PMIx (ret: %d)\n", rc);
	}
	fprintf(stderr, "Initializing succeeded\n");

	return 0;
}

int fini_pmix() {
	fprintf(stderr, "Finalizing PMIx...\n");
	PMIx_Finalize(NULL, 0);
	fprintf(stderr, "All done - Bye\n");

	return 0;
}
*/
import "C"
import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gvallee/go_util/pkg/util"
	"github.com/spf13/cobra"
	"github.com/sylabs/singularity/pkg/cmdline"
	pluginapi "github.com/sylabs/singularity/pkg/plugin"
	clicallback "github.com/sylabs/singularity/pkg/plugin/callback/cli"
	singularitycallback "github.com/sylabs/singularity/pkg/plugin/callback/runtime/engine/singularity"
	"github.com/sylabs/singularity/pkg/runtime/engine/config"
	singularity "github.com/sylabs/singularity/pkg/runtime/engine/singularity/config"
)

func installCallback(path string) error {
	// Create required stuff during "plugin install"
	// (eg: configuration file, setup ...). Be careful
	// during setup as this callback is executed with
	// root privileges.
	return nil
}

// Some global variables that allow us to create, track and manage the plugin
// state.
// Note that the plugin is currently  potentially loaded twice: once in the context
// of the CLI and once in the contexxt of the starter. As a result, the global
// variable CANNOT be used to pass state between these two spots, instead we
// rely on a /tmp/sypmix directory.
var enablePMIx bool

// Plugin is the only variable which a plugin MUST export.
// This symbol is accessed by the plugin framework to initialize the plugin.
var Plugin = pluginapi.Plugin{
	Manifest: pluginapi.Manifest{
		Name:        "github.com/gvallee/singularity-pmix-plugin",
		Author:      "Geoffroy Vallee",
		Version:     "0.1.0",
		Description: "PMIx plugin for Singularity",
	},
	Callbacks: []pluginapi.Callback{
		(clicallback.Command)(callbackExec),
		(clicallback.Command)(callbackRun),
		(singularitycallback.PostStartProcess)(callbackPMIxFinalize),
	},
}

func getBasedir() (string, error) {
	path := filepath.Join("/tmp", "sypmix")
	if !util.PathExists(path) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create %s\n", path)
		}
	}

	return path, nil
}

func createTempFile() error {
	pid := strconv.Itoa(os.Getpid())
	dir, err := getBasedir()
	if err != nil {
		return fmt.Errorf("failed to get base directory: %w", err)
	}
	file := filepath.Join(dir, pid)
	if util.FileExists(file) {
		return fmt.Errorf("file %s already exists", file)
	}
	err = ioutil.WriteFile(file, []byte("1"), 0644)
	if err != nil {
		return fmt.Errorf("impossible to create %s: %w", file, err)
	}

	return err
}

func checkTempFile() bool {
	pid := strconv.Itoa(os.Getpid())
	dir, err := getBasedir()
	if err != nil {
		return false
	}
	file := filepath.Join(dir, pid)
	if util.FileExists(file) {
		return true
	}
	return false
}

func deleteTempFile() error {
	pid := strconv.Itoa(os.Getpid())
	dir, err := getBasedir()
	if err != nil {
		return fmt.Errorf("failed to get base directory: %w", err)
	}
	file := filepath.Join(dir, pid)
	if util.FileExists(file) {
		err := os.RemoveAll(file)
		if err != nil {
			return fmt.Errorf("failed to delete %s: %w", file, err)
		}
	}

	return nil
}

func callbackRun(manager *cmdline.CommandManager) {
	runCmd := manager.GetCmd("run")
	if runCmd == nil {
		log.Printf("Could not find the 'run' command")
		return
	}

	manager.RegisterFlagForCmd(&cmdline.Flag{
		Value:        &enablePMIx,
		DefaultValue: false,
		Name:         "pmix",
		Usage:        "--pmix [true|false] to enable/disable PMIx support",
		Hidden:       false,
	}, runCmd)

	prerun := runCmd.PreRun
	runCmd.PreRun = func(c *cobra.Command, args []string) {
		if enablePMIx {
			err := createTempFile()
			if err != nil {
				fmt.Printf("Could not create temp file: %s", err)
				return
			}
			C.init_pmix()
		}
		if prerun != nil {
			prerun(c, args)
		}
	}

}

func callbackExec(manager *cmdline.CommandManager) {
	execCmd := manager.GetCmd("exec")
	if execCmd == nil {
		log.Printf("Could not find the 'exec' command")
		return
	}

	manager.RegisterFlagForCmd(&cmdline.Flag{
		Value:        &enablePMIx,
		DefaultValue: false,
		Name:         "pmix",
		Usage:        "--pmix [true|false] to enable/disable PMIx support",
		Hidden:       false,
	}, execCmd)

	prerun := execCmd.PreRun
	execCmd.PreRun = func(c *cobra.Command, args []string) {
		if enablePMIx {
			err := createTempFile()
			if err != nil {
				fmt.Printf("Could not create temp file: %s", err)
				return
			}
			C.init_pmix()
		}
		if prerun != nil {
			prerun(c, args)
		}
	}

}

func callbackPMIxFinalize(common *config.Common, pid int) error {
	_, ok := common.EngineConfig.(*singularity.EngineConfig)
	if !ok {
		fmt.Printf("Unexpected engine config")
		return fmt.Errorf("Unexpected engine config")
	}

	pmixEnabled := checkTempFile()
	if pmixEnabled {
		C.fini_pmix()
	}

	return nil
}
