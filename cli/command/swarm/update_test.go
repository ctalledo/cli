package swarm

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/docker/cli/internal/test"
	"github.com/docker/cli/internal/test/builders"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestSwarmUpdateErrors(t *testing.T) {
	testCases := []struct {
		name                  string
		args                  []string
		flags                 map[string]string
		swarmInspectFunc      func() (swarm.Swarm, error)
		swarmUpdateFunc       func(swarm swarm.Spec, flags swarm.UpdateFlags) error
		swarmGetUnlockKeyFunc func() (swarm.UnlockKeyResponse, error)
		expectedError         string
	}{
		{
			name:          "too-many-args",
			args:          []string{"foo"},
			expectedError: "accepts no arguments",
		},
		{
			name: "swarm-inspect-error",
			flags: map[string]string{
				flagTaskHistoryLimit: "10",
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return swarm.Swarm{}, errors.New("error inspecting the swarm")
			},
			expectedError: "error inspecting the swarm",
		},
		{
			name: "swarm-update-error",
			flags: map[string]string{
				flagTaskHistoryLimit: "10",
			},
			swarmUpdateFunc: func(swarm swarm.Spec, flags swarm.UpdateFlags) error {
				return errors.New("error updating the swarm")
			},
			expectedError: "error updating the swarm",
		},
		{
			name: "swarm-unlockkey-error",
			flags: map[string]string{
				flagAutolock: "true",
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *builders.Swarm(), nil
			},
			swarmGetUnlockKeyFunc: func() (swarm.UnlockKeyResponse, error) {
				return swarm.UnlockKeyResponse{}, errors.New("error getting unlock key")
			},
			expectedError: "error getting unlock key",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newUpdateCommand(
				test.NewFakeCli(&fakeClient{
					swarmInspectFunc:      tc.swarmInspectFunc,
					swarmUpdateFunc:       tc.swarmUpdateFunc,
					swarmGetUnlockKeyFunc: tc.swarmGetUnlockKeyFunc,
				}))
			if tc.args == nil {
				cmd.SetArgs([]string{})
			} else {
				cmd.SetArgs(tc.args)
			}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			for k, v := range tc.flags {
				assert.Check(t, cmd.Flags().Set(k, v))
			}
			assert.ErrorContains(t, cmd.Execute(), tc.expectedError)
		})
	}
}

func TestSwarmUpdate(t *testing.T) {
	swarmInfo := builders.Swarm()
	swarmInfo.ClusterInfo.TLSInfo.TrustRoot = "trustroot"

	testCases := []struct {
		name                  string
		args                  []string
		flags                 map[string]string
		swarmInspectFunc      func() (swarm.Swarm, error)
		swarmUpdateFunc       func(swarm swarm.Spec, flags swarm.UpdateFlags) error
		swarmGetUnlockKeyFunc func() (swarm.UnlockKeyResponse, error)
	}{
		{
			name: "noargs",
		},
		{
			name: "all-flags-quiet",
			flags: map[string]string{
				flagTaskHistoryLimit:    "10",
				flagDispatcherHeartbeat: "10s",
				flagCertExpiry:          "20s",
				flagExternalCA:          "protocol=cfssl,url=https://example.com.",
				flagMaxSnapshots:        "10",
				flagSnapshotInterval:    "100",
				flagAutolock:            "true",
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *swarmInfo, nil
			},
			swarmUpdateFunc: func(swarm swarm.Spec, flags swarm.UpdateFlags) error {
				if *swarm.Orchestration.TaskHistoryRetentionLimit != 10 {
					return errors.New("historyLimit not correctly set")
				}
				heartbeatDuration, err := time.ParseDuration("10s")
				if err != nil {
					return err
				}
				if swarm.Dispatcher.HeartbeatPeriod != heartbeatDuration {
					return errors.New("heartbeatPeriodLimit not correctly set")
				}
				certExpiryDuration, err := time.ParseDuration("20s")
				if err != nil {
					return err
				}
				if swarm.CAConfig.NodeCertExpiry != certExpiryDuration {
					return errors.New("certExpiry not correctly set")
				}
				if len(swarm.CAConfig.ExternalCAs) != 1 || swarm.CAConfig.ExternalCAs[0].CACert != "trustroot" {
					return errors.New("externalCA not correctly set")
				}
				if *swarm.Raft.KeepOldSnapshots != 10 {
					return errors.New("keepOldSnapshots not correctly set")
				}
				if swarm.Raft.SnapshotInterval != 100 {
					return errors.New("snapshotInterval not correctly set")
				}
				if !swarm.EncryptionConfig.AutoLockManagers {
					return errors.New("autolock not correctly set")
				}
				return nil
			},
		},
		{
			name: "autolock-unlock-key",
			flags: map[string]string{
				flagTaskHistoryLimit: "10",
				flagAutolock:         "true",
			},
			swarmUpdateFunc: func(swarm swarm.Spec, flags swarm.UpdateFlags) error {
				if *swarm.Orchestration.TaskHistoryRetentionLimit != 10 {
					return errors.New("historyLimit not correctly set")
				}
				return nil
			},
			swarmInspectFunc: func() (swarm.Swarm, error) {
				return *builders.Swarm(), nil
			},
			swarmGetUnlockKeyFunc: func() (swarm.UnlockKeyResponse, error) {
				return swarm.UnlockKeyResponse{
					UnlockKey: "unlock-key",
				}, nil
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cli := test.NewFakeCli(&fakeClient{
				swarmInspectFunc:      tc.swarmInspectFunc,
				swarmUpdateFunc:       tc.swarmUpdateFunc,
				swarmGetUnlockKeyFunc: tc.swarmGetUnlockKeyFunc,
			})
			cmd := newUpdateCommand(cli)
			if tc.args == nil {
				cmd.SetArgs([]string{})
			} else {
				cmd.SetArgs(tc.args)
			}
			for k, v := range tc.flags {
				assert.Check(t, cmd.Flags().Set(k, v))
			}
			cmd.SetOut(cli.OutBuffer())
			assert.NilError(t, cmd.Execute())
			golden.Assert(t, cli.OutBuffer().String(), fmt.Sprintf("update-%s.golden", tc.name))
		})
	}
}
