package main

import (
	"errors"
	"testing"

	"github.com/bancek/gabric/gabrictest"
)

func TestIPv6DataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if gt.RunNode(gt.Context(), ipv6) {
		t.Error("ipv6 = true, want false")
	}
}

func TestIPv6DataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("sh", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), ipv6); err == nil {
		t.Error("ipv6 error = nil, want error")
	}
}

func TestIPv6DataTransportError(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	if _, err := gt.RunNodeErr(ctx, ipv6); !errors.Is(err, sentinel) {
		t.Errorf("ipv6 error = %v, want %v", err, sentinel)
	}
}
