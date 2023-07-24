package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/viperx"
	"go.uber.org/zap"

	"go.infratographer.com/x/oauth2x"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/config"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/dataplaneapi"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/manager"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pubsub"
	"go.infratographer.com/loadbalancer-manager-haproxy/pkg/lbapi"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd starts loadbalancer-manager-haproxy service
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "starts the loadbalancer-manager-haproxy service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd.Context(), viper.GetViper())
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.PersistentFlags().StringSlice("events-topics", []string{}, "event topics to subscribe to")
	viperx.MustBindFlag(viper.GetViper(), "events.topics", runCmd.PersistentFlags().Lookup("events-topics"))

	runCmd.PersistentFlags().String("dataplane-user-name", "haproxy", "DataplaneAPI user name")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.user.name", runCmd.PersistentFlags().Lookup("dataplane-user-name"))

	runCmd.PersistentFlags().String("dataplane-user-pwd", "adminpwd", "DataplaneAPI user password")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.user.pwd", runCmd.PersistentFlags().Lookup("dataplane-user-pwd"))

	runCmd.PersistentFlags().String("dataplane-url", "http://127.0.0.1:5555/v2/", "DataplaneAPI base url")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.url", runCmd.PersistentFlags().Lookup("dataplane-url"))

	runCmd.PersistentFlags().String("base-haproxy-config", "", "Base config for haproxy")
	viperx.MustBindFlag(viper.GetViper(), "haproxy.config.base", runCmd.PersistentFlags().Lookup("base-haproxy-config"))

	runCmd.PersistentFlags().String("loadbalancerapi-url", "", "LoadbalancerAPI url")
	viperx.MustBindFlag(viper.GetViper(), "loadbalancerapi.url", runCmd.PersistentFlags().Lookup("loadbalancerapi-url"))

	runCmd.PersistentFlags().String("loadbalancer-id", "", "Loadbalancer ID to act on event changes")
	viperx.MustBindFlag(viper.GetViper(), "loadbalancer.id", runCmd.PersistentFlags().Lookup("loadbalancer-id"))

	events.MustViperFlagsForSubscriber(viper.GetViper(), runCmd.PersistentFlags())
	oauth2x.MustViperFlags(viper.GetViper(), runCmd.Flags())
}

func run(cmdCtx context.Context, v *viper.Viper) error {
	if err := validateMandatoryFlags(); err != nil {
		return err
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(cmdCtx)

	go func() {
		<-c
		cancel()
	}()

	mgr := &manager.Manager{
		Context:         ctx,
		Logger:          logger,
		DataPlaneClient: dataplaneapi.NewClient(viper.GetString("dataplane.url")),
		LBClient:        lbapi.NewClient(viper.GetString("loadbalancerapi.url")),
		ManagedLBID:     viper.GetString("loadbalancer.id"),
		BaseCfgPath:     viper.GetString("haproxy.config.base"),
	}

	// init lbapi client
	if config.AppConfig.OIDC.Client.Issuer != "" {
		oidcTS, err := oauth2x.NewClientCredentialsTokenSrc(ctx, config.AppConfig.OIDC.Client)
		if err != nil {
			logger.Fatalw("failed to create oauth2 token source", "error", err)
		}

		oauthHTTPClient := oauth2x.NewClient(ctx, oidcTS)
		mgr.LBClient = lbapi.NewClient(viper.GetString("loadbalancerapi.url"),
			lbapi.WithHTTPClient(oauthHTTPClient),
		)
	} else {
		mgr.LBClient = lbapi.NewClient(viper.GetString("loadbalancerapi.url"))
	}

	// init events subscriber
	subscriber, err := pubsub.NewSubscriber(
		ctx,
		config.AppConfig.Events.Subscriber,
		pubsub.WithMsgHandler(mgr.ProcessMsg))

	if err != nil {
		logger.Errorw("failed to create subscriber", zap.Error(err))
		return err
	}

	mgr.Subscriber = subscriber

	for _, topic := range viper.GetStringSlice("events.topics") {
		if err := mgr.Subscriber.Subscribe(topic); err != nil {
			logger.Errorw("failed to subscribe to changes topic", zap.String("topic", topic), zap.Error(err))
			return err
		}
	}

	defer func() {
		if err := mgr.Subscriber.Close(); err != nil {
			mgr.Logger.Errorw("failed to shutdown events subscriber", zap.Error(err))
		}
	}()

	if err := mgr.Run(); err != nil {
		logger.Fatalw("failed starting manager", "error", err)
	}

	return nil
}

// validateMandatoryFlags collects the mandatory flag validation
func validateMandatoryFlags() error {
	errs := []error{}

	if viper.GetString("events.subscriber.url") == "" {
		errs = append(errs, ErrSubscriberURLRequired)
	}

	if viper.GetString("events.subscriber.prefix") == "" {
		errs = append(errs, ErrSubscriberPrefixRequired)
	}

	if len(viper.GetStringSlice("events.topics")) < 1 {
		errs = append(errs, ErrSubscriberTopicsRequired)
	}

	if viper.GetString("haproxy.config.base") == "" {
		errs = append(errs, ErrHAProxyBaseConfigRequired)
	}

	if viper.GetString("loadbalancerapi.url") == "" {
		errs = append(errs, ErrLBAPIURLRequired)
	}

	if viper.GetString("loadbalancer.id") == "" {
		errs = append(errs, ErrLBIDRequired)
	} else {
		// check if the loadbalancer id is a valid gidx
		_, err := gidx.Parse(viper.GetString("loadbalancer.id"))
		if err != nil {
			errs = append(errs, ErrLBIDInvalid)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.Join(errs...) //nolint:goerr113
}
