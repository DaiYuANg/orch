package dnssvc

import (
	"context"
	"log/slog"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dnsx/dnsserver"
	"github.com/miekg/dns"
)

const workloadDNSForwardTimeout = 2 * time.Second

type forwardingHandler struct {
	resolver  *dnsserver.Resolver
	upstreams *list.List[string]
	logger    *slog.Logger
}

func newForwardingHandler(resolver *dnsserver.Resolver, upstreams *list.List[string], logger *slog.Logger) dns.Handler {
	if upstreams == nil {
		upstreams = list.NewList[string]()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &forwardingHandler{
		resolver:  resolver,
		upstreams: upstreams,
		logger:    logger,
	}
}

func (h *forwardingHandler) ServeDNS(writer dns.ResponseWriter, request *dns.Msg) {
	reply := new(dns.Msg)
	reply.SetReply(request)
	reply.Authoritative = false
	reply.RecursionAvailable = h.upstreams.Len() > 0

	if request.Opcode != dns.OpcodeQuery {
		reply.Rcode = dns.RcodeNotImplemented
		writeDNSReply(h.logger, writer, reply)
		return
	}
	if len(request.Question) == 0 {
		reply.Rcode = dns.RcodeFormatError
		writeDNSReply(h.logger, writer, reply)
		return
	}
	if h.resolver == nil {
		reply.Rcode = dns.RcodeRefused
		writeDNSReply(h.logger, writer, reply)
		return
	}

	resolution, err := h.resolver.Resolve(context.Background(), request.Question[0])
	if err != nil {
		h.logger.Error("dns resolve failed", "error", err, "name", request.Question[0].Name, "type", request.Question[0].Qtype)
		reply.Rcode = dns.RcodeServerFailure
		writeDNSReply(h.logger, writer, reply)
		return
	}
	if resolution.RCode == dns.RcodeRefused && h.upstreams.Len() > 0 {
		h.forward(writer, request)
		return
	}

	reply.Rcode = resolution.RCode
	reply.Authoritative = resolution.Authoritative
	reply.Answer = resolution.Answer
	reply.Ns = resolution.Authority
	reply.Extra = resolution.Extra
	writeDNSReply(h.logger, writer, reply)
}

func (h *forwardingHandler) forward(writer dns.ResponseWriter, request *dns.Msg) {
	var lastErr error
	h.upstreams.Range(func(_ int, upstream string) bool {
		query := request.Copy()
		query.RecursionDesired = true

		ctx, cancel := context.WithTimeout(context.Background(), workloadDNSForwardTimeout)
		defer cancel()

		response, _, err := (&dns.Client{Net: "udp", Timeout: workloadDNSForwardTimeout}).ExchangeContext(ctx, query, upstream)
		if err == nil && response != nil {
			if response.Truncated {
				response, _, err = (&dns.Client{Net: "tcp", Timeout: workloadDNSForwardTimeout}).ExchangeContext(ctx, query, upstream)
			}
			if err == nil && response != nil {
				response.RecursionAvailable = true
				writeDNSReply(h.logger, writer, response)
				return false
			}
		}
		lastErr = err
		return true
	})
	if lastErr != nil {
		h.logger.Warn("dns upstream forward failed", "error", lastErr, "name", request.Question[0].Name)
	}

	reply := new(dns.Msg)
	reply.SetReply(request)
	reply.RecursionAvailable = true
	reply.Rcode = dns.RcodeServerFailure
	writeDNSReply(h.logger, writer, reply)
}

func writeDNSReply(logger *slog.Logger, writer dns.ResponseWriter, msg *dns.Msg) {
	if err := writer.WriteMsg(msg); err != nil && logger != nil {
		logger.Warn("write dns response", "error", err)
	}
}

var _ dns.Handler = (*forwardingHandler)(nil)
