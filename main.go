package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"
	"os/exec"
	"time"

	aw "github.com/deanishe/awgo"
	"github.com/deanishe/awgo/update"
	"go.deanishe.net/fuzzy"
)

const (
	account = "honeycomb.io"
	repo    = "honeycomb/honeycomb-alfred"
)

var (
	cacheName   = "datasets.json"   // Filename of cached dataset list
	maxResults  = 200               // Number of results sent to Alfred
	maxCacheAge = 180 * time.Minute // How long to cache dataset list for

	// Command-line flags
	doDownload bool
	doCheck    bool
	query      string
	key        string

	// Workflow
	sopts  []fuzzy.Option
	wf     *aw.Workflow
	config *Configuration

	iconAvailable = &aw.Icon{Value: "update-available.png"}
)

type Configuration struct {
	APIHost, UIHost string
}

func init() {
	flag.BoolVar(&doDownload, "download", false, "retrieve list of datasets from Honeycomb")
	flag.StringVar(&key, "set", "", "set the honeycomb api key")
	flag.BoolVar(&doCheck, "check", false, "check for a new version")

	// Set some custom fuzzy search options
	sopts = []fuzzy.Option{
		fuzzy.AdjacencyBonus(10.0),
		fuzzy.LeadingLetterPenalty(-0.1),
		fuzzy.MaxLeadingLetterPenalty(-3.0),
		fuzzy.UnmatchedLetterPenalty(-0.5),
	}
	wf = aw.New(aw.HelpURL("https://www.honeycomb.io/"),
		aw.MaxResults(maxResults),
		aw.SortOptions(sopts...),
		update.GitHub(repo))
}

func run() {
	ctx := context.Background()
	wf.Args() // call to handle any magic actions

	config = &Configuration{}

	// Update config from environment variables
	if err := wf.Config.To(config); err != nil {
		wf.FatalError(err)
	}

	log.Printf("loaded: %#v", config)

	flag.Parse()

	if args := flag.Args(); len(args) > 0 {
		query = args[0]
	}

	if key != "" {
		if err := wf.Keychain.Set(account, key); err != nil {
			wf.FatalError(err)
		}
		return
	}

	if doCheck {
		wf.Configure(aw.TextErrors(true))
		log.Println("Checking for updates...")
		if err := wf.CheckForUpdate(); err != nil {
			wf.FatalError(err)
		}
		return
	}

	if doDownload {
		wf.Configure(aw.TextErrors(true))
		key, err := wf.Keychain.Get(account)
		if err != nil {
			wf.FatalError(err)
		}
		log.Printf("[main] downloading dataset list...")

		ui, err := url.Parse(config.UIHost)

		if err != nil {
			wf.FatalError(err)
		}

		api, err := url.Parse(config.APIHost)

		if err != nil {
			wf.FatalError(err)
		}

		log.Print(ui, api)

		datasets, err := fetchDatasets(ctx, ui, api, key)
		if err != nil {
			wf.FatalError(err)
		}
		if err := wf.Cache.StoreJSON(cacheName, datasets); err != nil {
			wf.FatalError(err)
		}
		log.Printf("[main] downloaded dataset list")
		return
	}

	if wf.UpdateCheckDue() && !wf.IsRunning("check-update") {
		log.Println("Running update check in background...")

		cmd := exec.Command(os.Args[0], "-check")
		if err := wf.RunInBackground("check-update", cmd); err != nil {
			log.Printf("Error starting update check: %s", err)
		}
	}

	if query == "" && wf.UpdateAvailable() {
		wf.Configure(aw.SuppressUIDs(true))
		wf.NewItem("Update available!").
			Subtitle("↩ to install").
			Autocomplete("workflow:update").
			Valid(false).
			Icon(iconAvailable)
	}

	log.Printf("[main] query=%s", query)

	// Try to load datasets
	datasets := []*Dataset{}
	if wf.Cache.Exists(cacheName) {
		if err := wf.Cache.LoadJSON(cacheName, &datasets); err != nil {
			wf.FatalError(err)
		}
	}

	// If the cache has expired, set Rerun (which tells Alfred to re-run the
	// workflow), and start the background update process if it isn't already
	// running.
	if wf.Cache.Expired(cacheName, maxCacheAge) {
		wf.Rerun(0.3)
		if !wf.IsRunning("download") {
			cmd := exec.Command(os.Args[0], "-download")
			if err := wf.RunInBackground("download", cmd); err != nil {
				wf.FatalError(err)
			}
		} else {
			log.Printf("download job already running.")
		}
		// Cache is also "expired" if it doesn't exist. So if there are no
		// cached data, show a corresponding message and exit.
		if len(datasets) == 0 {
			wf.NewItem("Downloading datasets").
				Icon(aw.IconInfo)
			wf.SendFeedback()
			return
		}
	}

	// Add results for cached datasets
	for _, d := range datasets {
		arg, err := d.URL()

		if err != nil {
			log.Printf("Bad URL: %#v", err)
			continue
		}

		wf.NewItem(d.Name).
			Subtitle(d.Team.Slug).
			Arg(arg).
			UID(d.UID()).
			Valid(true)
	}

	// Filter results against query if user entered one
	if query != "" {
		res := wf.Filter(query)
		log.Printf("[main] %d/%d datasets match %q", len(res), len(datasets), query)
	}

	// Convenience method that shows a warning if there are no results to show.
	// Alfred's default behaviour if no results are returned is to show its
	// fallback searches, which is also what it does if a workflow errors out.
	//
	// As such, it's a good idea to display a message in this situation,
	// otherwise the user can't tell if the workflow failed or simply found
	// no matching results.
	wf.WarnEmpty("No datasets found", "Try a different query?")

	// Send results/warning message to Alfred
	wf.SendFeedback()
}

func main() {
	wf.Run(run)
}
