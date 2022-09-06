package resolver

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
	"github.com/safing/portmaster/plugin/framework"
	"github.com/safing/portmaster/plugin/shared/proto"
	"github.com/safing/portmaster/plugin/shared/resolver"
	"github.com/txn2/txeh"
)

type HostsResolver struct {
	hostsFile *txeh.Hosts
}

func NewHostsResolver(file string) (*HostsResolver, error) {
	hostsFile, err := txeh.NewHosts(&txeh.HostsConfig{
		ReadFilePath: file,
	})

	if err != nil {
		return nil, err
	}

	go func() {
		ticker := time.NewTicker(time.Second * 10)
		for {
			select {
			case <-framework.Context().Done():
				return
			case <-ticker.C:
				if err := hostsFile.Reload(); err != nil {
					hclog.L().Error("failed to reload hosts file", "error", err)
				}
			}
		}
	}()

	return &HostsResolver{
		hostsFile: hostsFile,
	}, nil
}

func (resolver *HostsResolver) Resolve(ctx context.Context, question *proto.DNSQuestion, _ *proto.Connection) (*proto.DNSResponse, error) {
	if question.Class != dns.ClassINET {
		return nil, nil
	}

	// we can only handle requests for A or AAAA here.
	switch uint16(question.Type) {
	case dns.TypeA, dns.TypeAAAA:
	default:
		return nil, nil
	}

	ok, addr, _ := resolver.hostsFile.HostAddressLookup(question.GetName())
	if !ok {
		ok, addr, _ = resolver.hostsFile.HostAddressLookup(
			strings.TrimSuffix(question.GetName(), "."),
		)
	}

	if !ok {
		return nil, nil
	}

	ipaddr := net.ParseIP(addr)

	buildResponse := func(data []byte) *proto.DNSResponse {
		return &proto.DNSResponse{
			Rcode: dns.RcodeSuccess,
			Rrs: []*proto.DNSRR{
				{
					Name:  question.GetName(),
					Type:  question.GetType(),
					Class: question.GetClass(),
					Ttl:   60,
					Data:  data,
				},
			},
		}
	}

	switch uint16(question.Type) {
	case dns.TypeA:
		if ipv4 := ipaddr.To4(); ipv4 != nil {
			return buildResponse(ipv4), nil
		}
	case dns.TypeAAAA:
		if ipv6 := ipaddr.To16(); ipv6 != nil {
			return buildResponse(ipv6), nil
		}
	}

	return nil, nil
}

var _ resolver.Resolver = new(HostsResolver)
