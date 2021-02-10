package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"
)

const (
	datasets = "/1/datasets"
	team     = "/1/team_slug"
)

var (
	client *http.Client
)

func init() {
	client = &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 60 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
		},
	}
}

type Team struct {
	Slug   string `json:"team_slug"`
	UIHost *url.URL
}

type Dataset struct {
	Name string
	Slug string
	Team *Team
}

func (d *Dataset) UID() string {
	return fmt.Sprintf("%s-%s", d.Team.Slug, d.Slug)
}

func (d *Dataset) URL() (string, error) {
	path := path.Join(d.Team.Slug, "home", d.Slug)

	u, err := d.Team.UIHost.Parse(path)

	if err != nil {
		return "", err
	}

	return u.String(), nil
}

func fetchDatasets(ctx context.Context, ui, api *url.URL, key string) ([]*Dataset, error) {
	t, err := fetchTeam(ctx, ui, api, key)

	if err != nil {
		return nil, err
	}

	req, err := request(ctx, api, datasets, key)

	if err != nil {
		return nil, err
	}

	var datasets []*Dataset

	if err := response(ctx, req, &datasets); err != nil {
		return nil, err
	}

	for _, d := range datasets {
		d.Team = t
	}

	return datasets, nil
}

func fetchTeam(ctx context.Context, ui, api *url.URL, key string) (*Team, error) {
	req, err := request(ctx, api, team, key)

	if err != nil {
		return nil, err
	}

	var t Team

	if err := response(ctx, req, &t); err != nil {
		return nil, err
	}

	t.UIHost = ui

	return &t, nil
}

func request(ctx context.Context, u *url.URL, path, key string) (*http.Request, error) {
	url, err := u.Parse(path)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)

	if err != nil {
		return nil, err
	}

	req.Header.Add("X-Honeycomb-Team", key)

	return req, nil
}

func response(ctx context.Context, req *http.Request, v interface{}) error {
	res, err := client.Do(req)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return err
	}

	if err = json.Unmarshal(data, v); err != nil {
		return err
	}

	return nil
}
