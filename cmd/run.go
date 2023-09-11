package cmd

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"go.infratographer.com/x/events"
	"go.infratographer.com/x/gidx"
	"go.infratographer.com/x/viperx"
	"go.uber.org/zap"

	"go.infratographer.com/x/oauth2x"

	lbapi "go.infratographer.com/load-balancer-api/pkg/client"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/config"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/dataplaneapi"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/manager"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pubsub"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultDataplaneConnRetries       = 30
	defaultDataplaneConnRetryInterval = 1 * time.Second
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

	runCmd.PersistentFlags().StringSlice("change-topics", []string{}, "event change topics to subscribe to")
	viperx.MustBindFlag(viper.GetViper(), "change-topics", runCmd.PersistentFlags().Lookup("change-topics"))

	runCmd.PersistentFlags().String("dataplane-user-name", "haproxy", "DataplaneAPI user name")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.user.name", runCmd.PersistentFlags().Lookup("dataplane-user-name"))

	runCmd.PersistentFlags().String("dataplane-user-pwd", "adminpwd", "DataplaneAPI user password")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.user.pwd", runCmd.PersistentFlags().Lookup("dataplane-user-pwd"))

	runCmd.PersistentFlags().String("dataplane-url", "http://127.0.0.1:5555/v2/", "DataplaneAPI base url")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.url", runCmd.PersistentFlags().Lookup("dataplane-url"))

	runCmd.PersistentFlags().Int("dataplane-connect-retries", defaultDataplaneConnRetries, "DataplaneAPI connection retry attempts")
	viperx.MustBindFlag(viper.GetViper(), "dataplane-connect-retries", runCmd.PersistentFlags().Lookup("dataplane-connect-retries"))

	runCmd.PersistentFlags().Duration("dataplane-connect-retry-interval", defaultDataplaneConnRetryInterval, "DataplaneAPI connection retry interval")
	viperx.MustBindFlag(viper.GetViper(), "dataplane-connect-retry-interval", runCmd.PersistentFlags().Lookup("dataplane-connect-retry-interval"))

	runCmd.PersistentFlags().String("base-haproxy-config", "", "Base config for haproxy")
	viperx.MustBindFlag(viper.GetViper(), "haproxy.config.base", runCmd.PersistentFlags().Lookup("base-haproxy-config"))

	runCmd.PersistentFlags().String("loadbalancerapi-url", "", "LoadbalancerAPI url")
	viperx.MustBindFlag(viper.GetViper(), "loadbalancerapi.url", runCmd.PersistentFlags().Lookup("loadbalancerapi-url"))

	runCmd.PersistentFlags().String("loadbalancer-id", "", "Loadbalancer ID to act on event changes")
	viperx.MustBindFlag(viper.GetViper(), "loadbalancer.id", runCmd.PersistentFlags().Lookup("loadbalancer-id"))

	runCmd.PersistentFlags().Uint64("max-msg-process-attempts", 0, "maxiumum number of attempts at processing an event message")
	viperx.MustBindFlag(viper.GetViper(), "max-msg-process-attempts", runCmd.PersistentFlags().Lookup("max-msg-process-attempts"))

	events.MustViperFlags(viper.GetViper(), runCmd.PersistentFlags(), appName)
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

	managedLBID, err := gidx.Parse(viper.GetString("loadbalancer.id"))
	if err != nil {
		logger.Fatalw("failed to parse loadbalancer.id gidx: %w", err, "loadbalancerID", viper.GetString("loadbalancer.id"))
	}

	mgr := &manager.Manager{
		Context:                       ctx,
		Logger:                        logger,
		DataPlaneClient:               dataplaneapi.NewClient(viper.GetString("dataplane.url"), dataplaneapi.WithLogger(logger)),
		DataPlaneConnectRetries:       viper.GetInt("dataplane-connect-retries"),
		DataPlaneConnectRetryInterval: viper.GetDuration("dataplane-connect-retry-interval"),
		LBClient:                      lbapi.NewClient(viper.GetString("loadbalancerapi.url")),
		ManagedLBID:                   managedLBID,
		BaseCfgPath:                   viper.GetString("haproxy.config.base"),
	}

	logger.Infow("Initializing...", zap.String("loadbalancerID", viper.GetString("loadbalancer.id")))

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

	// generate a random queuegroup name
	// this is to prevent multiple instances of this service from receiving the same message
	// and processing it
	config.AppConfig.Events.NATS.QueueGroup = generateQueueGroupName()

	events, err := events.NewConnection(config.AppConfig.Events, events.WithLogger(logger))
	if err != nil {
		logger.Fatalw("failed to create events connection", "error", err)
	}

	// init events subscriber
	subscriber := pubsub.NewSubscriber(
		ctx,
		events,
		pubsub.WithMsgHandler(mgr.ProcessMsg),
		pubsub.WithLogger(logger),
		pubsub.WithMaxMsgProcessAttempts(viper.GetUint64("max-msg-process-attempts")),
	)

	mgr.Subscriber = subscriber

	for _, topic := range viper.GetStringSlice("change-topics") {
		if err := mgr.Subscriber.Subscribe(topic); err != nil {
			logger.Errorw("failed to subscribe to change topic", zap.String("topic", topic), zap.Error(err))
			return err
		}
	}

	defer func() {
		const shutdownTimeout = 10 * time.Second
		ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)

		defer cancel()

		_ = events.Shutdown(ctx)
	}()

	if err := mgr.Run(); err != nil {
		logger.Fatalw("failed starting manager", "error", err)
	}

	return nil
}

// validateMandatoryFlags collects the mandatory flag validation
func validateMandatoryFlags() error {
	errs := []error{}

	if len(viper.GetStringSlice("change-topics")) < 1 {
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
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.Join(errs...) //nolint:goerr113
}

// generateQueueGroupName generates a random queue group name with prefix lbmanager-haproxy-
func generateQueueGroupName() string {
	const rlen = 10

	alphaNum := []rune("abcdefghijklmnopqrstuvwxyz1234567890")
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]rune, rlen)

	for i := range b {
		b[i] = alphaNum[r.Intn(len(alphaNum))]
	}

	return fmt.Sprintf("lbmanager-haproxy-%s-", string(b))
}
