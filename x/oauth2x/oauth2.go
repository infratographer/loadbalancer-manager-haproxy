package oauth2x

import (
	"context"
	"net/http"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"go.infratographer.com/x/viperx"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// NewClientCredentialsTokenSrc returns an oauth2 client credentials token source
func NewClientCredentialsTokenSrc(ctx context.Context, cfg Config) oauth2.TokenSource {
	ccCfg := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
	}

	return ccCfg.TokenSource(ctx)
}

// NewClient returns a http client using requested token source
func NewClient(ctx context.Context, tokenSrc oauth2.TokenSource) *http.Client {
	return oauth2.NewClient(ctx, tokenSrc)
}

// Config handles reading in all the config values available
// for setting up an oauth2 configuration
type Config struct {
	ClientID     string `mapstructure:"id"`
	ClientSecret string `mapstructure:"secret"`
	TokenURL     string `mapstructure:"tokenURL"`
}

// MustViperFlags adds oidc oauth2 client credentials config to the provided flagset and binds to viper
func MustViperFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.String("oidc-client-id", "", "expected oidc client identifier")
	viperx.MustBindFlag(v, "oidc.client.id", flags.Lookup("oidc-client-id"))

	flags.String("oidc-client-secret", "", "expected oidc client secret")
	viperx.MustBindFlag(v, "oidc.client.secret", flags.Lookup("oidc-client-secret"))

	flags.String("oidc-client-token-url", "", "expected oidc token url")
	viperx.MustBindFlag(v, "oidc.client.tokenURL", flags.Lookup("oidc-client-token-url"))
}