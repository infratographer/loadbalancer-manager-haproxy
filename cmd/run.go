package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/dataplaneapi"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/manager"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pubsub"
	"go.infratographer.com/loadbalancer-manager-haproxy/pkg/lbapi"
	"go.infratographer.com/x/gidx"
	"go.uber.org/zap"

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

	runCmd.PersistentFlags().String("nats-url", "", "NATS server connection url")
	viperBindFlag("nats.url", runCmd.PersistentFlags().Lookup("nats-url"))

	runCmd.PersistentFlags().String("nats-creds", "", "Path to the file containing the NATS credentials")
	viperBindFlag("nats.creds", runCmd.PersistentFlags().Lookup("nats-creds"))

	runCmd.PersistentFlags().String("nats-subject-prefix", "com.infratographer", "prefix for NATS subjects")
	viperBindFlag("nats.subject-prefix", runCmd.PersistentFlags().Lookup("nats-subject-prefix"))

	runCmd.PersistentFlags().StringSlice("nats-subjects", []string{"changes.*.load-balancer"}, "NATS subjects to subscribe to")
	viperBindFlag("nats.subjects", runCmd.PersistentFlags().Lookup("nats-subjects"))

	runCmd.PersistentFlags().String("dataplane-user-name", "haproxy", "DataplaneAPI user name")
	viperBindFlag("dataplane.user.name", runCmd.PersistentFlags().Lookup("dataplane-user-name"))

	runCmd.PersistentFlags().String("dataplane-user-pwd", "adminpwd", "DataplaneAPI user password")
	viperBindFlag("dataplane.user.pwd", runCmd.PersistentFlags().Lookup("dataplane-user-pwd"))

	runCmd.PersistentFlags().String("dataplane-url", "http://127.0.0.1:5555/v2/", "DataplaneAPI base url")
	viperBindFlag("dataplane.url", runCmd.PersistentFlags().Lookup("dataplane-url"))

	runCmd.PersistentFlags().String("base-haproxy-config", "", "Base config for haproxy")
	viperBindFlag("haproxy.config.base", runCmd.PersistentFlags().Lookup("base-haproxy-config"))

	runCmd.PersistentFlags().String("loadbalancerapi-url", "", "LoadbalancerAPI url")
	viperBindFlag("loadbalancerapi.url", runCmd.PersistentFlags().Lookup("loadbalancerapi-url"))

	runCmd.PersistentFlags().String("loadbalancer-id", "", "Loadbalancer ID to act on event changes")
	viperBindFlag("loadbalancer.id", runCmd.PersistentFlags().Lookup("loadbalancer-id"))
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
		Context:     ctx,
		Logger:      logger,
		ManagedLBID: viper.GetString("loadbalancer.id"),
		BaseCfgPath: viper.GetString("haproxy.config.base"),
	}

	// init other components
	mgr.DataPlaneClient = dataplaneapi.NewClient(viper.GetString("dataplane.url"))
	mgr.LBClient = lbapi.NewClient(viper.GetString("loadbalancerapi.url"))

	// setup, connect to nats and subscribe to subjects
	mgr.NatsClient = pubsub.NewNatsClient(ctx, viper.GetString("nats.url"),
		pubsub.WithUserCredentials(viper.GetString("nats.creds")),
		pubsub.WithLogger(logger),
		pubsub.WithMsgHandler(mgr.ProcessMsg),
	)

	if err := mgr.NatsClient.Connect(); err != nil {
		logger.Error("failed to connect to nats server", zap.Error(err))
		return err
	}

	subjects := viper.GetStringSlice("nats.subjects")
	prefix := viper.GetString("nats.subject-prefix")

	for _, subject := range subjects {
		prefixedSubjectQueue := fmt.Sprintf("%s.%s", prefix, subject)
		if err := mgr.NatsClient.Subscribe(prefixedSubjectQueue); err != nil {
			logger.Errorw("failed to subscribe to queue ", zap.String("subject", prefixedSubjectQueue))
			return err
		}
	}

	defer func() {
		if err := mgr.NatsClient.Close(); err != nil {
			mgr.Logger.Errorw("failed to shutdown nats client", zap.Error(err))
		}
	}()

	if err := mgr.Run(); err != nil {
		logger.Fatalw("failed starting manager", "error", err)
	}

	return nil
}

// validateMandatoryFlags collects the mandatory flag validation
func validateMandatoryFlags() error {
	errs := []string{}

	if viper.GetString("nats.url") == "" {
		errs = append(errs, ErrNATSURLRequired.Error())
	}

	if viper.GetString("nats.creds") == "" {
		errs = append(errs, ErrNATSAuthRequired.Error())
	}

	if viper.GetString("nats.subject-prefix") == "" {
		errs = append(errs, ErrNATSSubjectPrefixRequired.Error())
	}

	if viper.GetString("haproxy.config.base") == "" {
		errs = append(errs, ErrHAProxyBaseConfigRequired.Error())
	}

	if viper.GetString("loadbalancerapi.url") == "" {
		errs = append(errs, ErrLBAPIURLRequired.Error())
	}

	if viper.GetString("loadbalancer.id") == "" {
		errs = append(errs, ErrLBIDRequired.Error())
	}

	// check if the loadbalancer id is a valid gidx
	_, err := gidx.Parse(viper.GetString("loadbalancer.id"))
	if err != nil {
		errs = append(errs, ErrLBIDInvalid.Error())
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf(strings.Join(errs, "\n")) //nolint:goerr113
}
