package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/pkg"

	"github.com/nats-io/nats.go"
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

	runCmd.PersistentFlags().String("nats-nkey", "", "Path to the file containing the NATS nkey keypair")
	viperBindFlag("nats.nkey", runCmd.PersistentFlags().Lookup("nats-nkey"))

	runCmd.PersistentFlags().String("nats-subject", "loadbalancer-manager-haproxy", "NATS subject to subscribe to")
	viperBindFlag("nats.subject", runCmd.PersistentFlags().Lookup("nats-subject"))

	runCmd.PersistentFlags().String("dataplane-user-name", "haproxy", "DataplaneAPI user name")
	viperBindFlag("dataplane.user.name", runCmd.PersistentFlags().Lookup("dataplane-user-name"))

	runCmd.PersistentFlags().String("dataplane-user-pwd", "adminpwd", "DataplaneAPI user password")
	viperBindFlag("dataplane.user.pwd", runCmd.PersistentFlags().Lookup("dataplane-user-pwd"))

	runCmd.PersistentFlags().String("dataplane-config", "", "DataplaneAPI config")
	viperBindFlag("dataplane.config", runCmd.PersistentFlags().Lookup("dataplane-config"))

	runCmd.PersistentFlags().String("base-haproxy-config", "", "Base config for haproxy")
	viperBindFlag("haproxy.config.base", runCmd.PersistentFlags().Lookup("base-haproxy-config"))

	runCmd.PersistentFlags().String("loadbalancerapi-url", "", "LoadbalancerAPI url")
	viperBindFlag("loadbalancerapi.url", runCmd.PersistentFlags().Lookup("loadbalancerapi-url"))
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

	natsConn, err := nats.Connect(
		viper.GetString("nats.url"),
		newNatsOptions()...,
	)
	if err != nil {
		logger.Fatalw("failed connecting to nats", "error", err)
	}
	defer natsConn.Close()

	// init other components

	mgr := &pkg.Manager{
		Logger:   logger,
		NatsConn: natsConn,
	}

	if err := mgr.Run(ctx); err != nil {
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

	if viper.GetString("nats.nkey") == "" {
		errs = append(errs, ErrNATSAuthRequired.Error())
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf(strings.Join(errs, "\n")) //nolint:goerr113
}

func newNatsOptions() []nats.Option {
	opts := []nats.Option{}

	logger.Debug("enabling nkey authentication")
	nkey := viper.GetString("nats.nkey")
	opt, err := nats.NkeyOptionFromSeed(nkey)
	if err != nil {
		logger.Fatalw("failed to configure nats nkey auth", "error", err)
	}
	opts = append(opts, opt)

	return opts
}
