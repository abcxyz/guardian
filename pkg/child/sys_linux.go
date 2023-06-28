// Copyright 2023 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package child

import "syscall"

// setSysProcAttr sets the sysProcAttr.
func (c *Child) setSysProcAttr() {
	c.cmd.SysProcAttr = &syscall.SysProcAttr{
		// kill children if parent is dead
		Pdeathsig: syscall.SIGKILL,
		// set process group ID
		Setpgid: true,
	}
}

// setCancel sets the Cancel behavior for a child process.
func (c *Child) setCancel() {
	c.cmd.Cancel = func() error {
		return c.cmd.Process.Signal(syscall.SIGQUIT) //nolint:wrapcheck // Want passthrough
	}
}
