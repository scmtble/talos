// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package alibabacloud contains the AlibabaCloud implementation of the [platform.Platform].

package alibabacloud

import (
	"context"
	"fmt"
	"log"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type AlibabaCloud struct{}

func (a *AlibabaCloud) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	log.Printf("fetching machine config from: %q", UserDataEndpoint)

	return download.Download(ctx, UserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

func (a *AlibabaCloud) KernelArgs(_ string, _ quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

func (a *AlibabaCloud) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching alibabacloud instance config")

	metadata, err := a.getMetadata(ctx)
	if err != nil {
		return err
	}

	networkConfig, err := a.ParseMetadata(metadata)
	if err != nil {
		return err
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (a *AlibabaCloud) ParseMetadata(metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{
		TimeServers: []network.TimeServerSpecSpec{
			{
				NTPServers:  metadata.NTPServers,
				ConfigLayer: network.ConfigPlatform,
			},
		},
	}

	if metadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	if len(metadata.Nameservers) > 0 {
		var dnsIPs []netip.Addr

		for _, dnsIP := range metadata.Nameservers {
			if ip, err := netip.ParseAddr(dnsIP); err == nil {
				dnsIPs = append(dnsIPs, ip)
			}
		}

		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:  dnsIPs,
			ConfigLayer: network.ConfigPlatform,
		})
	}

	var publicIPs []string
	if metadata.PublicIPv4 != "" {
		publicIPs = append(publicIPs, metadata.PublicIPv4)
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     a.Name(),
		Hostname:     metadata.Hostname,
		Region:       metadata.Region,
		Zone:         metadata.Zone,
		InstanceType: metadata.InstanceType,
		InstanceID:   metadata.InstanceID,
		ProviderID:   fmt.Sprintf("%s.%s", metadata.Region, metadata.InstanceID),
	}

	return networkConfig, nil
}

func (a *AlibabaCloud) Name() string {
	return "alibabacloud"
}

func (a *AlibabaCloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}
