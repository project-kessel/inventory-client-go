package common

import (
	"crypto/tls"
	"time"
)

type Config struct {
	Insecure           bool
	Url                string
	HttpUrl            string
	EnableOIDCAuth     bool
	clientId           string
	clientSecret       string
	authServerTokenUrl string
	TlsConfig          *tls.Config
	Timeout            time.Duration
	TokenHTTPTimeout   time.Duration
}

type ClientOptions func(*Config)

func WithAuthEnabled(clientId string, clientSecret string, authServerTokenUrl string) ClientOptions {
	return func(c *Config) {
		c.EnableOIDCAuth = true
		c.clientId = clientId
		c.clientSecret = clientSecret
		c.authServerTokenUrl = authServerTokenUrl
	}
}

func WithgRPCUrl(url string) ClientOptions {
	return func(c *Config) {
		c.Url = url
	}
}

func WithHTTPUrl(url string) ClientOptions {
	return func(c *Config) {
		c.HttpUrl = url
	}
}

func WithTLSInsecure(insecure bool) ClientOptions {
	return func(c *Config) {
		c.Insecure = insecure
	}
}

func WithHTTPTLSConfig(tlsConfig *tls.Config) ClientOptions {
	return func(c *Config) {
		c.Insecure = false
		c.TlsConfig = tlsConfig
	}
}

func WithTimeout(timeout time.Duration) ClientOptions {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

func NewConfig(options ...func(*Config)) *Config {
	svr := &Config{}
	for _, o := range options {
		o(svr)
	}
	return svr
}
