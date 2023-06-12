package dnsprovider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/caarlos0/env/v6"
	"github.com/ionos-cloud/external-dns-ionos-plugin/cmd/plugin/init/configuration"
	"github.com/ionos-cloud/external-dns-ionos-plugin/internal/ionos"
	"github.com/ionos-cloud/external-dns-ionos-plugin/internal/ionoscloud"
	"github.com/ionos-cloud/external-dns-ionos-plugin/internal/ionoscore"
	"github.com/ionos-cloud/external-dns-ionos-plugin/pkg/endpoint"
	"github.com/ionos-cloud/external-dns-ionos-plugin/pkg/provider"
	log "github.com/sirupsen/logrus"
)

type IONOSProviderFactory func(domainFilter endpoint.DomainFilter, ionosConfig *ionos.Configuration, dryRun bool) (provider.Provider, error)

func setDefaults(apiEndpointURL, authHeader string, ionosConfig *ionos.Configuration) {
	if ionosConfig.APIEndpointURL == "" {
		ionosConfig.APIEndpointURL = apiEndpointURL
	}
	if ionosConfig.AuthHeader == "" {
		ionosConfig.AuthHeader = authHeader
	}
}

var IonosCoreProviderFactory = func(domainFilter endpoint.DomainFilter, ionosConfig *ionos.Configuration, dryRun bool) (provider.Provider, error) {
	setDefaults("https://api.hosting.ionos.com/dns", "X-API-Key", ionosConfig)
	return ionoscore.NewProvider(domainFilter, ionosConfig, dryRun)
}

var IonosCloudProviderFactory = func(domainFilter endpoint.DomainFilter, ionosConfig *ionos.Configuration, dryRun bool) (provider.Provider, error) {
	setDefaults("https://dns.de-fra.ionos.com", "Bearer", ionosConfig)
	return ionoscloud.NewProvider(domainFilter, ionosConfig, dryRun)
}

func Init(config configuration.Config) (provider.Provider, error) {
	var domainFilter endpoint.DomainFilter
	createMsg := "Creating IONOS provider with "

	if config.RegexDomainFilter != "" {
		createMsg += fmt.Sprintf("Regexp domain filter: '%s', ", config.RegexDomainFilter)
		if config.RegexDomainExclusion != "" {
			createMsg += fmt.Sprintf("with exclusion: '%s', ", config.RegexDomainExclusion)
		}
		domainFilter = endpoint.NewRegexDomainFilter(
			regexp.MustCompile(config.RegexDomainFilter),
			regexp.MustCompile(config.RegexDomainExclusion),
		)
	} else {
		if config.DomainFilter != nil && len(config.DomainFilter) > 0 {
			createMsg += fmt.Sprintf("Domain filter: '%s', ", strings.Join(config.DomainFilter, ","))
		}
		if config.ExcludeDomains != nil && len(config.ExcludeDomains) > 0 {
			createMsg += fmt.Sprintf("Exclude domain filter: '%s', ", strings.Join(config.ExcludeDomains, ","))
		}
		domainFilter = endpoint.NewDomainFilterWithExclusions(config.DomainFilter, config.ExcludeDomains)
	}

	createMsg = strings.TrimSuffix(createMsg, ", ")
	if strings.HasSuffix(createMsg, "with ") {
		createMsg += "no kind of domain filters"
	}
	log.Info(createMsg)
	if config.DryRun {
		log.Warn("***** Dry run enabled, DNS records will not be created or deleted *****")
	}
	ionosConfig := ionos.Configuration{}
	if err := env.Parse(&ionosConfig); err != nil {
		return nil, fmt.Errorf("reading ionos ionosConfig failed: %v", err)
	}
	createProvider := detectProvider(&ionosConfig)
	provider, err := createProvider(domainFilter, &ionosConfig, config.DryRun)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize IONOS provider: %v", err)
	}
	return provider, nil
}

func detectProvider(ionosConfig *ionos.Configuration) IONOSProviderFactory {
	token := ionosConfig.APIKey
	split := strings.Split(token, ".")
	providerType := IonosCoreProviderFactory
	if len(split) == 3 {
		tokenBytes, err := base64.RawStdEncoding.DecodeString(split[1])
		if err == nil {
			var tokenMap map[string]interface{}
			err = json.Unmarshal(tokenBytes, &tokenMap)
			if err == nil {
				if tokenMap["iss"] == "ionoscloud" {
					providerType = IonosCloudProviderFactory
				}
			}
		}
	}
	return providerType
}
