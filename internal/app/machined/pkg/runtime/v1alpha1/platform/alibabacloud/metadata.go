package alibabacloud

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	MetadataEndpoint = "http://100.100.100.200/latest/meta-data"
	UserDataEndpoint = "http://100.100.100.200/latest/user-data"
)

// MetadataConfig represents a metadata AWS instance.
type MetadataConfig struct {
	Hostname     string   `json:"hostname,omitempty"`
	InstanceID   string   `json:"instance-id,omitempty"`
	InstanceType string   `json:"instance-type,omitempty"`
	PublicIPv4   string   `json:"public-ipv4,omitempty"`
	Region       string   `json:"region,omitempty"`
	Zone         string   `json:"zone,omitempty"`
	NTPServers   []string `json:"ntp-servers"`
	Nameservers  []string `json:"name-servers"`
}

//nolint:gocyclo
func (a *AlibabaCloud) getMetadata(ctx context.Context) (*MetadataConfig, error) {
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	getMetadataKey := func(key string) (string, error) {
		// https://www.alibabacloud.com/help/doc-detail/49122.htm
		url := fmt.Sprintf("%s/%s", MetadataEndpoint, key)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("metadata service returned status code %d", resp.StatusCode)
		}

		defer resp.Body.Close()
		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		return string(respBytes), nil
	}

	var (
		metadata MetadataConfig
		err      error
	)
	if metadata.Hostname, err = getMetadataKey("hostname"); err != nil {
		return nil, err
	}

	if metadata.InstanceType, err = getMetadataKey("instance/instance-type"); err != nil {
		return nil, err
	}

	if metadata.InstanceID, err = getMetadataKey("instance-id"); err != nil {
		return nil, err
	}

	if metadata.PublicIPv4, err = getMetadataKey("public-ipv4"); err != nil {
		return nil, err
	}

	if metadata.Region, err = getMetadataKey("region-id"); err != nil {
		return nil, err
	}

	if metadata.Zone, err = getMetadataKey("zone-id"); err != nil {
		return nil, err
	}

	if ntpServers, err := getMetadataKey("ntp-conf/ntp-servers"); err != nil {
		return nil, err
	} else {
		metadata.NTPServers = strings.Split(ntpServers, "\n")
	}

	if nameServers, err := getMetadataKey("dns-conf/nameservers"); err != nil {
		return nil, err
	} else {
		metadata.Nameservers = strings.Split(nameServers, "\n")
	}

	return &metadata, nil
}
