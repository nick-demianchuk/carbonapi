package zipper

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dgryski/go-expirecache"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/bookingcom/carbonapi/pkg/backend"
	"github.com/bookingcom/carbonapi/pkg/types"
)

type tldPrefix struct {
	prefix        string
	segments      []string
	segmentsCount int
}

func probeTopLevelDomains(TLDCache *expirecache.Cache, TLDPrefixes []tldPrefix, backends []backend.Backend, period int32, ms *PrometheusMetrics) {
	probeTicker := time.NewTicker(time.Duration(period) * time.Second) // TODO: The ticker resources are never freed
	for {
		topLevelDomainCache := make(map[string][]*backend.Backend)
		for _, prefix := range TLDPrefixes {
			bs := getBackendsForPrefix(prefix, backends, topLevelDomainCache)
			for i := range bs {
				topLevelDomains, err := getTopLevelDomains(*bs[i], prefix)
				ms.TLDCacheProbeReqTotal.Inc()
				if err != nil {
					// this could add a lot of noise to logs
					// lg.Error("failed to probe TLD cache for a backend", zap.Error(err), zap.String("backend", app.Backends[i].GetServerAddress()))
					ms.TLDCacheProbeErrors.Inc()
				}
				for _, topLevelDomain := range topLevelDomains {
					topLevelDomainCache[topLevelDomain] = append(topLevelDomainCache[topLevelDomain], bs[i])
				}
			}
		}
		for tld, num := range topLevelDomainCache {
			if utf8.ValidString(tld) {
				ms.TLDCacheHostsPerDomain.WithLabelValues(tld).Set(float64(len(num)))
			}
		}
		TLDCache.Set("tlds", topLevelDomainCache, 0, 2*period)

		<-probeTicker.C
	}
}

func (p *tldPrefix) query() string {
	query := "*"
	if p.prefix != "" {
		query = p.prefix + ".*"
	}
	return query
}

// getBackendsForPrefix returns the backends that need to be queried in order to populate TLD cache for the prefix.
// It reuses already fetched tlds to find out about the info. If no info is there, it returns all the backends.
func getBackendsForPrefix(prefix tldPrefix, backends []backend.Backend, tldCache map[string][]*backend.Backend) []*backend.Backend {
	for i := prefix.segmentsCount; i > 0; i-- {
		p := strings.Join(prefix.segments[:i], ".")
		if filteredBackends, ok := tldCache[p]; ok {
			return filteredBackends
		}
	}
	allBackends := make([]*backend.Backend, len(backends))
	for i := range backends {
		allBackends[i] = &backends[i]
	}
	return allBackends
}

// Returns the backend's top-level domains.
func getTopLevelDomains(backend backend.Backend, prefix tldPrefix) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request := types.NewFindRequest(prefix.query())
	matches, err := backend.Find(ctx, request)
	if err != nil {
		return nil, errors.Wrap(err, "find request failed")
	}
	var paths []string
	for _, m := range matches.Matches {
		paths = append(paths, m.Path)
	}
	return paths, nil
}

// InitTLDPrefixes gets unprocessed prefixes read from config, validates them (discards invalid ones),
// and sorts them in ascending order
func InitTLDPrefixes(logger *zap.Logger, cfgPrefixes []string) []tldPrefix {
	tldPrefixes := []tldPrefix{
		// We should always have empty prefix to get default TLDs
		{prefix: "", segments: nil, segmentsCount: 0},
	}

	for _, p := range cfgPrefixes {
		segments := strings.Split(p, ".")
		var invalid bool
		for _, s := range segments {
			if s == "" {
				invalid = true
				break
			}
		}
		if invalid {
			logger.Warn("tld prefix invalid", zap.String("prefix", p))
			continue
		}
		tldPrefixes = append(tldPrefixes, tldPrefix{
			prefix:        p,
			segments:      segments,
			segmentsCount: len(segments),
		})
	}
	// Sorting to avoid involving unnecessary backends and to optimize identifying query TLDs
	sort.Slice(tldPrefixes, func(i, j int) bool {
		return tldPrefixes[i].segmentsCount < tldPrefixes[j].segmentsCount
	})
	var uniqueTLDPrefixes []tldPrefix
	for i := range tldPrefixes {
		if i == 0 || tldPrefixes[i].prefix != tldPrefixes[i-1].prefix {
			uniqueTLDPrefixes = append(uniqueTLDPrefixes, tldPrefixes[i])
		}
	}
	return uniqueTLDPrefixes
}

func getTargetTopLevelDomain(target string, prefixes []tldPrefix) string {
	tld := strings.SplitN(target, ".", 2)[0]
	for i := len(prefixes) - 1; i >= 0; i-- {
		p := prefixes[i]
		if strings.HasPrefix(target, p.prefix) {
			splitTarget := strings.SplitN(target, ".", p.segmentsCount+2)  // prefix + ns | rest
			foundTLD := strings.Join(splitTarget[:p.segmentsCount+1], ".") // prefix + ns
			return foundTLD
		}
	}
	return tld
}

func (app *App) filterBackendByTopLevelDomain(targets []string) []backend.Backend {
	targetTlds := make([]string, 0, len(targets))
	for _, target := range targets {
		targetTlds = append(targetTlds, getTargetTopLevelDomain(target, app.TLDPrefixes))
	}

	bs := app.filterByTopLevelDomain(app.Backends, targetTlds)
	if len(bs) > 0 {
		return bs
	}
	return app.Backends
}

func (app *App) filterByTopLevelDomain(backends []backend.Backend, targetTLDs []string) []backend.Backend {
	bs := make([]backend.Backend, 0)
	allTLDBackends := make([]*backend.Backend, 0)

	topLevelDomainCache, _ := app.TopLevelDomainCache.Get("tlds")
	tldCache := make(map[string][]*backend.Backend)
	if x, ok := topLevelDomainCache.(map[string][]*backend.Backend); ok {
		tldCache = x
	}

	if tldCache == nil {
		return backends
	}
	alreadyAddedBackends := make(map[string]bool)
	for _, target := range targetTLDs {
		tldBackends := tldCache[target]
		for _, backend := range tldBackends {
			a := *backend
			if !alreadyAddedBackends[a.GetServerAddress()] {
				alreadyAddedBackends[a.GetServerAddress()] = true
				allTLDBackends = append(allTLDBackends, backend)
			}
		}
	}
	for _, tldBackend := range allTLDBackends {
		bs = append(bs, *tldBackend)
	}

	return bs
}
