package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"time"

	aw "github.com/deanishe/awgo"
	"go.deanishe.net/fuzzy"
)

const (
	account = "honeycomb.io"
)

var (
	cacheName   = "datasets.json"   // Filename of cached dataset list
	maxResults  = 200               // Number of results sent to Alfred
	maxCacheAge = 180 * time.Minute // How long to cache dataset list for

	// Command-line flags
	doDownload bool
	query      string
	key        string

	// Workflow
	sopts []fuzzy.Option
	wf    *aw.Workflow
)

func init() {
	flag.BoolVar(&doDownload, "download", false, "retrieve list of datasets from Honeycomb")
	flag.StringVar(&key, "set", "", "set the honeycomb api key")

	// Set some custom fuzzy search options
	sopts = []fuzzy.Option{
		fuzzy.AdjacencyBonus(10.0),
		fuzzy.LeadingLetterPenalty(-0.1),
		fuzzy.MaxLeadingLetterPenalty(-3.0),
		fuzzy.UnmatchedLetterPenalty(-0.5),
	}
	wf = aw.New(aw.HelpURL("https://www.honeycomb.io/"),
		aw.MaxResults(maxResults),
		aw.SortOptions(sopts...))
}

func set(key string) error {
	return wf.Keychain.Set(account, key)
}

func run() {
	ctx := context.Background()
	wf.Args() // call to handle any magic actions
	flag.Parse()

	if args := flag.Args(); len(args) > 0 {
		query = args[0]
	}

	if key != "" {
		if err := set(key); err != nil {
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
		datasets, err := fetchDatasets(ctx, key)
		if err != nil {
			wf.FatalError(err)
		}
		if err := wf.Cache.StoreJSON(cacheName, datasets); err != nil {
			wf.FatalError(err)
		}
		log.Printf("[main] downloaded dataset list")
		return
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
		wf.NewItem(d.Name).
			Subtitle(d.Team.Slug).
			Arg(d.URL()).
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
