package router

import (
	"context"
	"regexp"
	"runtime"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/common/platform/filesystem"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/features/routing"
)

type Rule struct {
	Tag       string
	RuleTag   string
	Balancer  *Balancer
	Condition Condition
	Webhook   *WebhookNotifier
}

func (r *Rule) GetTag() (string, error) {
	if r.Balancer != nil {
		return r.Balancer.PickOutbound()
	}
	return r.Tag, nil
}

// Apply checks rule matching of current routing context.
func (r *Rule) Apply(ctx routing.Context) bool {
	return r.Condition.Apply(ctx)
}

func (rr *RoutingRule) BuildCondition() (Condition, error) {
	conds := NewConditionChan()

	if len(rr.InboundTag) > 0 {
		conds.Add(NewInboundTagMatcher(rr.InboundTag))
	}

	if len(rr.Networks) > 0 {
		conds.Add(NewNetworkMatcher(rr.Networks))
	}

	if len(rr.Protocol) > 0 {
		conds.Add(NewProtocolMatcher(rr.Protocol))
	}

	if rr.PortList != nil {
		conds.Add(NewPortMatcher(rr.PortList, MatcherAsType_Target))
	}

	if rr.SourcePortList != nil {
		conds.Add(NewPortMatcher(rr.SourcePortList, MatcherAsType_Source))
	}

	if rr.LocalPortList != nil {
		conds.Add(NewPortMatcher(rr.LocalPortList, MatcherAsType_Local))
	}

	if rr.VlessRouteList != nil {
		conds.Add(NewPortMatcher(rr.VlessRouteList, MatcherAsType_VlessRoute))
	}

	if len(rr.UserEmail) > 0 {
		conds.Add(NewUserMatcher(rr.UserEmail))
	}

	if len(rr.Attributes) > 0 {
		configuredKeys := make(map[string]*regexp.Regexp)
		for key, value := range rr.Attributes {
			configuredKeys[strings.ToLower(key)] = regexp.MustCompile(value)
		}
		conds.Add(&AttributeMatcher{configuredKeys})
	}

	if len(rr.Geoip) > 0 {
		cond, err := NewIPMatcher(rr.Geoip, MatcherAsType_Target)
		if err != nil {
			return nil, err
		}
		conds.Add(cond)
		rr.Geoip = nil
		runtime.GC()
	}

	if len(rr.SourceGeoip) > 0 {
		cond, err := NewIPMatcher(rr.SourceGeoip, MatcherAsType_Source)
		if err != nil {
			return nil, err
		}
		conds.Add(cond)
		rr.SourceGeoip = nil
		runtime.GC()
	}

	if len(rr.LocalGeoip) > 0 {
		cond, err := NewIPMatcher(rr.LocalGeoip, MatcherAsType_Local)
		if err != nil {
			return nil, err
		}
		conds.Add(cond)
		errors.LogWarning(context.Background(), "Due to some limitations, in UDP connections, localIP is always equal to listen interface IP, so \"localIP\" rule condition does not work properly on UDP inbound connections that listen on all interfaces")
		rr.LocalGeoip = nil
		runtime.GC()
	}

	if len(rr.Domain) > 0 {
		var matcher *DomainMatcher
		var err error
		// Check if domain matcher cache is provided via environment
		domainMatcherPath := platform.NewEnvFlag(platform.MphCachePath).GetValue(func() string { return "" })

		if domainMatcherPath != "" {
			matcher, err = GetDomainMatcherWithRuleTag(domainMatcherPath, rr.RuleTag)
			if err != nil {
				return nil, errors.New("failed to build domain condition from cached MphDomainMatcher").Base(err)
			}
			errors.LogDebug(context.Background(), "MphDomainMatcher loaded from cache for ", rr.RuleTag, " rule tag)")

		} else {
			matcher, err = NewMphMatcherGroup(rr.Domain)
			if err != nil {
				return nil, errors.New("failed to build domain condition with MphDomainMatcher").Base(err)
			}
			errors.LogDebug(context.Background(), "MphDomainMatcher is enabled for ", len(rr.Domain), " domain rule(s)")
		}
		conds.Add(matcher)
		rr.Domain = nil
		runtime.GC()
	}

	if len(rr.Process) > 0 {
		conds.Add(NewProcessNameMatcher(rr.Process))
	}

	if conds.Len() == 0 {
		return nil, errors.New("this rule has no effective fields").AtWarning()
	}

	return conds, nil
}

// Build builds the balancing rule
func (br *BalancingRule) Build(ohm outbound.Manager, dispatcher routing.Dispatcher) (*Balancer, error) {
	switch strings.ToLower(br.Strategy) {
	case "leastping":
		return &Balancer{
			selectors:   br.OutboundSelector,
			strategy:    &LeastPingStrategy{},
			fallbackTag: br.FallbackTag,
			ohm:         ohm,
		}, nil
	case "roundrobin":
		return &Balancer{
			selectors:   br.OutboundSelector,
			strategy:    &RoundRobinStrategy{FallbackTag: br.FallbackTag},
			fallbackTag: br.FallbackTag,
			ohm:         ohm,
		}, nil
	case "leastload":
		i, err := br.StrategySettings.GetInstance()
		if err != nil {
			return nil, err
		}
		s, ok := i.(*StrategyLeastLoadConfig)
		if !ok {
			return nil, errors.New("not a StrategyLeastLoadConfig").AtError()
		}
		leastLoadStrategy := NewLeastLoadStrategy(s)
		return &Balancer{
			selectors:   br.OutboundSelector,
			ohm:         ohm,
			fallbackTag: br.FallbackTag,
			strategy:    leastLoadStrategy,
		}, nil
	case "random":
		fallthrough
	case "":
		return &Balancer{
			selectors:   br.OutboundSelector,
			ohm:         ohm,
			fallbackTag: br.FallbackTag,
			strategy:    &RandomStrategy{FallbackTag: br.FallbackTag},
		}, nil
	default:
		return nil, errors.New("unrecognized balancer type")
	}
}

func GetDomainMatcherWithRuleTag(domainMatcherPath string, ruleTag string) (*DomainMatcher, error) {
	f, err := filesystem.NewFileReader(domainMatcherPath)
	if err != nil {
		return nil, errors.New("failed to load file: ", domainMatcherPath).Base(err)
	}
	defer f.Close()

	g, err := LoadGeoSiteMatcher(f, ruleTag)
	if err != nil {
		return nil, errors.New("failed to load file:", domainMatcherPath).Base(err)
	}
	return &DomainMatcher{
		Matchers: g,
	}, nil

}
