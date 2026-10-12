// Copyright 2026 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package plugintest provides Terraform working directory fixtures that try to
// make Terraform load a provisioner plugin from the (untrusted) working
// directory. They are shared by the unit, command and end-to-end tests so that
// every layer is tested against the same layouts.
package plugintest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// RejectedErr is a substring of the error returned when a working directory
// contains a provisioner plugin.
const RejectedErr = "found provisioner plugins"

// Case is a Terraform working directory layout.
type Case struct {
	// Name is the test case name.
	Name string

	// Config is the content of main.tf.
	Config string

	// Plugins are the paths, relative to the working directory, of fake
	// provisioner plugin executables. When executed, each one creates the
	// marker file passed to Setup and exits.
	Plugins []string

	// PluginPathDirs, if set, are written to .terraform/plugin_path as
	// directories relative to the working directory, as if terraform init
	// -plugin-dir had been run.
	PluginPathDirs []string

	// WantRejected is true if Guardian must refuse to run in this directory.
	WantRejected bool

	// WantRemoved are paths, relative to the working directory, that Guardian
	// must remove before running terraform init.
	WantRemoved []string

	// TerraformRunsPlugin is true if, without Guardian's checks, terraform init
	// followed by terraform validate executes one of the Plugins.
	TerraformRunsPlugin bool
}

const (
	localExecConfig = `resource "terraform_data" "x" {
  provisioner "local-exec" {
    command = "true"
  }
}
`
	fileConfig = `resource "terraform_data" "x" {
  provisioner "file" {
    content     = "x"
    destination = "/tmp/guardian-plugintest"
  }
}
`
	customConfig = `resource "terraform_data" "x" {
  provisioner "evil" {}
}
`
	noProvisionerConfig = `resource "terraform_data" "x" {}
`
)

// Cases returns the provisioner plugin fixtures.
func Cases() []*Case {
	return []*Case{
		{
			// The only case that must keep working: a built-in provisioner and no
			// plugins anywhere in the working directory.
			Name:   "T0_builtin_provisioner_no_plugins",
			Config: localExecConfig,
		},
		{
			Name:                "T1_custom_plugin_in_working_dir",
			Config:              customConfig,
			Plugins:             []string{"terraform-provisioner-evil"},
			WantRejected:        true,
			TerraformRunsPlugin: true,
		},
		{
			// A plugin with the name of a built-in provisioner overrides it.
			Name:                "T2_builtin_override_in_working_dir",
			Config:              localExecConfig,
			Plugins:             []string{"terraform-provisioner-local-exec"},
			WantRejected:        true,
			TerraformRunsPlugin: true,
		},
		{
			// .terraform/plugin_path replaces every default plugin directory.
			Name:                "T3_plugin_path",
			Config:              fileConfig,
			Plugins:             []string{"evil/terraform-provisioner-file"},
			PluginPathDirs:      []string{"evil"},
			WantRemoved:         []string{".terraform/plugin_path"},
			TerraformRunsPlugin: true,
		},
		{
			// Terraform only starts plugins the configuration references, but a
			// plugin in the working directory is rejected regardless.
			Name:         "T4_unreferenced_plugin_in_working_dir",
			Config:       noProvisionerConfig,
			Plugins:      []string{"terraform-provisioner-evil"},
			WantRejected: true,
		},
		{
			Name:                "T5_versioned_builtin_override_in_working_dir",
			Config:              localExecConfig,
			Plugins:             []string{"terraform-provisioner-local-exec_v1.0.0"},
			WantRejected:        true,
			TerraformRunsPlugin: true,
		},
	}
}

// Setup writes the case's layout into dir. Every fake plugin, if executed,
// creates the file at marker and exits with a non-zero status.
func (c *Case) Setup(tb testing.TB, dir, marker string) {
	tb.Helper()

	write(tb, filepath.Join(dir, "main.tf"), []byte(c.Config), 0o600)

	script := fmt.Sprintf("#!/bin/sh\ntouch %q\nexit 1\n", marker)
	for _, p := range c.Plugins {
		write(tb, filepath.Join(dir, p), []byte(script), 0o700)
	}

	if len(c.PluginPathDirs) > 0 {
		abs := make([]string, 0, len(c.PluginPathDirs))
		for _, d := range c.PluginPathDirs {
			abs = append(abs, filepath.Join(dir, d))
		}
		b, err := json.Marshal(abs)
		if err != nil {
			tb.Fatal(err)
		}
		write(tb, filepath.Join(dir, ".terraform", "plugin_path"), b, 0o600)
	}
}

// CheckRemoved returns an error if any of the case's WantRemoved paths still
// exist in dir. Command tests call it when terraform init is invoked.
func (c *Case) CheckRemoved(dir string) error {
	for _, p := range c.WantRemoved {
		_, err := os.Lstat(filepath.Join(dir, p))
		if err == nil {
			return fmt.Errorf("expected %q to be removed before terraform init", p)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to stat %q: %w", p, err)
		}
	}
	return nil
}

func write(tb testing.TB, p string, b []byte, mode os.FileMode) {
	tb.Helper()

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(p, b, mode); err != nil {
		tb.Fatal(err)
	}
}
