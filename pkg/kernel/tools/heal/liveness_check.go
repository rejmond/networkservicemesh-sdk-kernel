// Copyright (c) 2022 Cisco and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package heal contains an implementation of LivenessChecker.
package heal

import (
	"context"
	"net"
	"time"

	"github.com/go-ping/ping"
	"github.com/networkservicemesh/api/pkg/api/networkservice"
	"github.com/networkservicemesh/api/pkg/api/networkservice/mechanisms/kernel"
	"github.com/networkservicemesh/sdk/pkg/tools/log"
)

const (
	defaultTimeout = time.Second
	packetCount    = 4
)

// KernelLivenessCheck is an implementation of heal.LivenessCheck. It sends ICMP
// ping and checks reply. Returns false if didn't get reply.
func KernelLivenessCheck(deadlineCtx context.Context, conn *networkservice.Connection) bool {
	if mechanism := conn.GetMechanism().GetType(); mechanism != kernel.MECHANISM {
		log.FromContext(deadlineCtx).Warnf("ping is not supported for mechanism %v", mechanism)
		return true
	}

	deadline, ok := deadlineCtx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultTimeout)
	}

	addrCount := len(conn.GetContext().GetIpContext().GetDstIpAddrs())
	timeout := time.Until(deadline) / time.Duration(addrCount+1)

	// This function requires string argument. Works fine with empty string.
	pinger, err := ping.NewPinger("")
	if err != nil {
		log.FromContext(deadlineCtx).Errorf("Failed to create pinger: %s", err.Error())
	}
	pinger.SetPrivileged(true)
	pinger.Timeout = timeout
	pinger.Count = packetCount

	for _, cidr := range conn.GetContext().GetIpContext().GetDstIpAddrs() {
		addr, _, err := net.ParseCIDR(cidr)
		if err != nil {
			log.FromContext(deadlineCtx).Errorf("ParseCIDR failed: %s", err.Error())
			return false
		}

		ipAddr := &net.IPAddr{IP: addr}
		pinger.SetIPAddr(ipAddr)
		err = pinger.Run()
		if err != nil {
			log.FromContext(deadlineCtx).Errorf("Ping failed: %s", err.Error())
			return false
		}

		if pinger.Statistics().PacketsRecv == 0 {
			return false
		}
	}

	select {
	case <-deadlineCtx.Done():
		return false
	default:
		return true
	}
}
