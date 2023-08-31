package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.infratographer.com/x/viperx"

	"go.infratographer.com/loadbalancer-manager-haproxy/internal/dataplaneapi"
)

// checkDataplaneCmd checks the connection to the dataplaneapi
var checkDataplaneCmd = &cobra.Command{
	Use:   "check_dataplane",
	Short: "checks the connection to the dataplaneapi",
	RunE: func(cmd *cobra.Command, args []string) error {
		return checkDataPlane(cmd.Context(), viper.GetViper())
	},
}

const (
	defaultRetryLimit    = 3
	defaultRetryInterval = 1 * time.Second
)

func init() {
	rootCmd.AddCommand(checkDataplaneCmd)

	checkDataplaneCmd.PersistentFlags().String("dataplane-user-name", "haproxy", "DataplaneAPI user name")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.user.name", checkDataplaneCmd.PersistentFlags().Lookup("dataplane-user-name"))

	checkDataplaneCmd.PersistentFlags().String("dataplane-user-pwd", "adminpwd", "DataplaneAPI user password")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.user.pwd", checkDataplaneCmd.PersistentFlags().Lookup("dataplane-user-pwd"))

	checkDataplaneCmd.PersistentFlags().String("dataplane-url", "http://127.0.0.1:5555/v2/", "DataplaneAPI base url")
	viperx.MustBindFlag(viper.GetViper(), "dataplane.url", checkDataplaneCmd.PersistentFlags().Lookup("dataplane-url"))

	checkDataplaneCmd.PersistentFlags().Int("retries", defaultRetryLimit, "Number of attempts to verify connection to DataplaneAPI")
	viperx.MustBindFlag(viper.GetViper(), "retries", checkDataplaneCmd.PersistentFlags().Lookup("retries"))

	checkDataplaneCmd.PersistentFlags().Duration("retry-interval", defaultRetryInterval, "Interval between checks")
	viperx.MustBindFlag(viper.GetViper(), "retry-interval", checkDataplaneCmd.PersistentFlags().Lookup("retry-interval"))
}

func checkDataPlane(ctx context.Context, viper *viper.Viper) error {
	client := dataplaneapi.NewClient(viper.GetString("dataplane.url"))

	if err := client.WaitForDataPlaneReady(
		ctx,
		viper.GetInt("retries"),
		viper.GetDuration("retry-interval"),
	); err != nil {
		logger.Fatalw("dataplane api is not ready", "error", err)
	}

	return nil
}
