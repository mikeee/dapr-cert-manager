package app

import (
	"fmt"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/spf13/cobra"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/diagridio/dapr-cert-manager-helper/cmd/app/options"
	"github.com/diagridio/dapr-cert-manager-helper/pkg/controller"
	"github.com/diagridio/dapr-cert-manager-helper/pkg/trustanchor"
)

const (
	helpOutput = "Operator for managing a dapr trust bundle using a cert-manager Certificate resource"
)

// NewCommand will return a new command instance for the
// dapr-cert-manager-helper operator.
func NewCommand() *cobra.Command {
	opts := options.New()

	cmd := &cobra.Command{
		Use:   "dapr-cert-manager-helper",
		Short: helpOutput,
		Long:  helpOutput,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}

			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("error adding corev1 to scheme: %w", err)
			}
			if err := cmapi.AddToScheme(scheme); err != nil {
				return fmt.Errorf("error adding cert-manager scheme: %w", err)
			}

			mgr, err := ctrl.NewManager(opts.RestConfig, ctrl.Options{
				Scheme:                        scheme,
				LeaderElection:                true,
				LeaderElectionNamespace:       opts.DaprNamespace,
				LeaderElectionID:              "dapr-cert-manager-helper",
				LeaderElectionReleaseOnCancel: true,
				ReadinessEndpointName:         "/readyz",
				HealthProbeBindAddress:        fmt.Sprintf("0.0.0.0:%d", opts.ReadyzPort),
				MetricsBindAddress:            fmt.Sprintf("0.0.0.0:%d", opts.MetricsPort),
				Logger:                        opts.Logr.WithName("manager"),
				Namespace:                     opts.DaprNamespace,
				LeaderElectionResourceLock:    "leases",
			})
			if err != nil {
				return fmt.Errorf("failed to create manager: %w", err)
			}

			ctx := ctrl.SetupSignalHandler()
			var taSource x509bundle.Source
			if len(opts.TrustAnchorFilePath) > 0 {
				ta := trustanchor.New(trustanchor.Options{
					Log:             opts.Logr,
					TrustBundlePath: opts.TrustAnchorFilePath,
				})
				if err := mgr.Add(ta); err != nil {
					return err
				}
				taSource = ta
			}

			if err := controller.AddTrustBundle(ctx, mgr, controller.Options{
				Log:                        opts.Logr,
				DaprNamespace:              opts.DaprNamespace,
				TrustBundleCertificateName: opts.TrustBundleCertificateName,
				TrustAnchor:                taSource,
			}); err != nil {
				return err
			}

			// Start all runnables and controller
			return mgr.Start(ctx)
		},
	}

	opts = opts.Prepare(cmd)

	return cmd
}
