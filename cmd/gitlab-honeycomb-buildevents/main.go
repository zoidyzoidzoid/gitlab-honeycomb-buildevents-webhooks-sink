package main

import (
	"log"
	"os"
	"strconv"

	"github.com/zoidbergwill/gitlab-honeycomb-buildevents-webhooks-sink/internal/hook"

	"github.com/honeycombio/libhoney-go"
	"github.com/spf13/cobra"
)

// Version is the default value that should be overridden in the
// build/release process.
var Version = "dev"

func commandRoot(cfg *libhoney.Config) (*cobra.Command, bool) {
	root := &cobra.Command{
		Version: Version,
		Use:     "buildevents",
		Short:   "buildevents creates events for your CI builds",
		Long: `
The buildevents executable creates Honeycomb events and tracing information
about your Continuous Integration builds.`,
	}

	root.PersistentFlags().StringVarP(&cfg.APIKey, "apikey", "k", "", "[env.BUILDEVENT_APIKEY] the Honeycomb authentication token")
	if apikey, ok := os.LookupEnv("BUILDEVENT_APIKEY"); ok {
		// https://github.com/spf13/viper/issues/461#issuecomment-366831834
		err := root.PersistentFlags().Lookup("apikey").Value.Set(apikey)
		if err != nil {
			log.Fatalf("failed to configure `apikey`: %s", err)
		}
	}

	root.PersistentFlags().StringVarP(&cfg.Dataset, "dataset", "d", "buildevents", "[env.BUILDEVENT_DATASET] the name of the Honeycomb dataset to which to send these events")
	if dataset, ok := os.LookupEnv("BUILDEVENT_DATASET"); ok {
		err := root.PersistentFlags().Lookup("dataset").Value.Set(dataset)
		if err != nil {
			log.Fatalf("failed to configure `dataset`: %s", err)
		}
	}

	root.PersistentFlags().StringVarP(&cfg.APIHost, "apihost", "a", "https://api.honeycomb.io", "[env.BUILDEVENT_APIHOST] the hostname for the Honeycomb API server to which to send this event")
	if apihost, ok := os.LookupEnv("BUILDEVENT_APIHOST"); ok {
		err := root.PersistentFlags().Lookup("apihost").Value.Set(apihost)
		if err != nil {
			log.Fatalf("failed to configure `apihost`: %s", err)
		}
	}

	debug := root.PersistentFlags().Bool("debug", false, "[env.DEBUG] set the debug logging to true")
	if debugEnv, ok := os.LookupEnv("DEBUG"); ok {
		debugEnvParsed, err := strconv.ParseBool(debugEnv)
		if err != nil {
			log.Fatalf("failed to configure `debug`: %s", err)
		}
		debug = &debugEnvParsed
	}

	return root, *debug
}

func main() {
	defer libhoney.Close()
	var config libhoney.Config

	root, debug := commandRoot(&config)

	// Do the work
	if err := root.Execute(); err != nil {
		libhoney.Close()
		os.Exit(1)
	}

	log.SetOutput(os.Stdout)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	l, err := hook.New(hook.Config{
		Version:         Version,
		ListenAddr:      ":" + port,
		HookSecret:      "",
		Debug:           debug,
		HoneycombConfig: &config,
	})
	if err != nil {
		log.Fatalf("failed to setup hook listener: %s", err)
	}

	log.Printf("Starting server on http://%s\n", l.HTTPServer.Addr)
	log.Fatal(l.ListenAndServe())
}
