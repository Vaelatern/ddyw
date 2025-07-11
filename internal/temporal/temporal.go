package temporal

import (
	"crypto/tls"
	"log/slog"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
)

func Logger() log.Logger {
	return log.NewStructuredLogger(
		slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		})))
}

func EasyClient(logger log.Logger) (client.Client, error) {
	temporalHost, haveValue := os.LookupEnv("TEMPORAL_ADDRESS")
	if !haveValue {
		temporalHost = "localhost:7233"
	}
	temporalNamespace, haveValue := os.LookupEnv("TEMPORAL_NAMESPACE")
	if !haveValue {
		temporalNamespace = "default"
	}
	_, haveValue = os.LookupEnv("TEMPORAL_TLS")
	var temporalTLS *tls.Config
	if haveValue {
		temporalTLS = &tls.Config{}
	}

	clientOptions := client.Options{
		HostPort:    temporalHost,
		Credentials: nil,
		Logger:      logger,
		Namespace:   temporalNamespace,
	}
	clientOptions.ConnectionOptions.TLS = temporalTLS

	temporalClient, err := client.Dial(clientOptions)
	return temporalClient, err
}
