package dktest

import (
	"context"
	"io"
	"testing"
)

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

import (
	"github.com/dhui/dktest/mockdockerclient"
)

const (
	imageName = "dktestFakeImageName"
)

var (
	containerInfo = ContainerInfo{}
)

// ready functions
func alwaysReady(ContainerInfo) bool { return true }
func neverReady(ContainerInfo) bool  { return false }

func testErr(t *testing.T, err error, expectErr bool) {
	t.Helper()
	if err == nil && expectErr {
		t.Error("Expected an error but didn't get one")
	} else if err != nil && !expectErr {
		t.Error("Got unexpected error:", err)
	}
}

func TestPullImage(t *testing.T) {
	successReader := mockdockerclient.MockReader{Err: io.EOF}

	testCases := []struct {
		name      string
		client    mockdockerclient.ImageAPIClient
		expectErr bool
	}{
		{name: "success", client: mockdockerclient.ImageAPIClient{
			PullResp: mockdockerclient.MockReadCloser{MockReader: successReader}}, expectErr: false},
		{name: "pull error", client: mockdockerclient.ImageAPIClient{}, expectErr: true},
		{name: "read error", client: mockdockerclient.ImageAPIClient{
			PullResp: mockdockerclient.MockReadCloser{
				MockReader: mockdockerclient.MockReader{Err: mockdockerclient.Err},
			}}, expectErr: false},
		{name: "close error", client: mockdockerclient.ImageAPIClient{
			PullResp: mockdockerclient.MockReadCloser{
				MockReader: successReader,
				MockCloser: mockdockerclient.MockCloser{Err: mockdockerclient.Err},
			}}, expectErr: false},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pullImage(ctx, t, &tc.client, imageName)
			testErr(t, err, tc.expectErr)
		})
	}
}

func TestRunImage(t *testing.T) {
	_, portBindingsNoIP, err := nat.ParsePortSpecs([]string{"8181:80"})
	if err != nil {
		t.Fatal("Error parsing port bindings:", err)
	}
	_, portBindingsIPZeros, err := nat.ParsePortSpecs([]string{"0.0.0.0:8181:80"})
	if err != nil {
		t.Fatal("Error parsing port bindings:", err)
	}
	_, portBindingsDiffIP, err := nat.ParsePortSpecs([]string{"10.0.0.1:8181:80"})
	if err != nil {
		t.Fatal("Error parsing port bindings:", err)
	}

	successCreateResp := &container.ContainerCreateCreatedBody{}
	successInspectResp := &types.ContainerJSON{}
	successInspectRespWithPortBindingNoIP := &types.ContainerJSON{NetworkSettings: &types.NetworkSettings{
		NetworkSettingsBase: types.NetworkSettingsBase{Ports: portBindingsNoIP},
	}}
	successInspectRespWithPortBindingIPZeros := &types.ContainerJSON{NetworkSettings: &types.NetworkSettings{
		NetworkSettingsBase: types.NetworkSettingsBase{Ports: portBindingsIPZeros},
	}}
	successInspectRespWithPortBindingDiffIP := &types.ContainerJSON{NetworkSettings: &types.NetworkSettings{
		NetworkSettingsBase: types.NetworkSettingsBase{Ports: portBindingsDiffIP},
	}}

	testCases := []struct {
		name      string
		client    mockdockerclient.ContainerAPIClient
		opts      Options
		expectErr bool
	}{
		{name: "success", client: mockdockerclient.ContainerAPIClient{
			CreateResp: successCreateResp, InspectResp: successInspectResp}, expectErr: false},
		{name: "success - with port binding no ip", client: mockdockerclient.ContainerAPIClient{
			CreateResp: successCreateResp, InspectResp: successInspectRespWithPortBindingNoIP}, expectErr: false},
		{name: "success - with port binding ip 0.0.0.0", client: mockdockerclient.ContainerAPIClient{
			CreateResp: successCreateResp, InspectResp: successInspectRespWithPortBindingIPZeros}, expectErr: false},
		{name: "success - with port binding diff ip", client: mockdockerclient.ContainerAPIClient{
			CreateResp: successCreateResp, InspectResp: successInspectRespWithPortBindingDiffIP}, expectErr: false},
		{name: "create error", client: mockdockerclient.ContainerAPIClient{InspectResp: successInspectResp},
			expectErr: true},
		{name: "start error", client: mockdockerclient.ContainerAPIClient{
			CreateResp: successCreateResp, StartErr: mockdockerclient.Err, InspectResp: successInspectResp,
		}, expectErr: true},
		{name: "inspect error", client: mockdockerclient.ContainerAPIClient{
			CreateResp: successCreateResp,
		}, expectErr: true},
		{name: "no network settings error", client: mockdockerclient.ContainerAPIClient{
			CreateResp: successCreateResp, InspectResp: successInspectResp}, opts: Options{PortRequired: true},
			expectErr: true},
		{name: "no ports error", client: mockdockerclient.ContainerAPIClient{
			CreateResp:  successCreateResp,
			InspectResp: &types.ContainerJSON{NetworkSettings: &types.NetworkSettings{}}},
			opts: Options{PortRequired: true}, expectErr: true},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runImage(ctx, t, &tc.client, imageName, tc.opts)
			testErr(t, err, tc.expectErr)
		})
	}
}

func TestStopContainer(t *testing.T) {
	testCases := []struct {
		name   string
		client mockdockerclient.ContainerAPIClient
	}{
		{name: "success", client: mockdockerclient.ContainerAPIClient{}},
		{name: "stop error", client: mockdockerclient.ContainerAPIClient{StopErr: mockdockerclient.Err}},
		{name: "remove error", client: mockdockerclient.ContainerAPIClient{RemoveErr: mockdockerclient.Err}},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stopContainer(ctx, t, &tc.client, containerInfo)
		})
	}
}

func TestWaitContainerReady(t *testing.T) {
	canceledCtx, cancelFunc := context.WithCancel(context.Background())
	cancelFunc()

	testCases := []struct {
		name        string
		ctx         context.Context
		readyFunc   func(ContainerInfo) bool
		expectReady bool
	}{
		{name: "nil readyFunc", ctx: canceledCtx, readyFunc: nil, expectReady: true},
		{name: "ready", ctx: context.Background(), readyFunc: alwaysReady, expectReady: true},
		{name: "not ready", ctx: canceledCtx, readyFunc: neverReady, expectReady: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if ready := waitContainerReady(tc.ctx, t, containerInfo, tc.readyFunc); ready && !tc.expectReady {
				t.Error("Expected container to not be ready but it was")
			} else if !ready && tc.expectReady {
				t.Error("Expected container to ready but it wasn't")
			}
		})
	}
}