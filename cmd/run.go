package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/dataplaneapi"
	"go.infratographer.com/loadbalancer-manager-haproxy/internal/manager"
	"go.infratographer.com/loadbalancer-manager-haproxy/pkg/lbapi"

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

	runCmd.PersistentFlags().String("nats-creds", "", "Path to the file containing the NATS credentials")
	viperBindFlag("nats.creds", runCmd.PersistentFlags().Lookup("nats-creds"))

	runCmd.PersistentFlags().String("nats-subject", "loadbalancer-manager-haproxy", "NATS subject to subscribe to")
	viperBindFlag("nats.subject", runCmd.PersistentFlags().Lookup("nats-subject"))

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
		nats.UserCredentials(viper.GetString("nats.creds")),
	)
	if err != nil {
		logger.Fatalw("failed connecting to nats", "error", err)
	}
	defer natsConn.Close()

	// init other components
	dpc := dataplaneapi.NewClient(viper.GetString("dataplane.url"))
	lbc := lbapi.NewClient(viper.GetString("loadbalancerapi.url"))

	mgr := &manager.Manager{
		Context:         ctx,
		Logger:          logger,
		NatsConn:        natsConn,
		DataPlaneClient: dpc,
		LBClient:        lbc,
		BaseCfgPath:     viper.GetString("haproxy.config.base"),
	}

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

	if viper.GetString("haproxy.config.base") == "" {
		errs = append(errs, ErrHAProxyBaseConfigRequired.Error())
	}

	if viper.GetString("loadbalancerapi.url") == "" {
		errs = append(errs, ErrLBAPIURLRequired.Error())
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf(strings.Join(errs, "\n")) //nolint:goerr113
}
