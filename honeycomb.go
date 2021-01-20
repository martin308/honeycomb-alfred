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
	api, _ = url.Parse("https://api.honeycomb.io")
	web, _ = url.Parse("https://ui.honeycomb.io")
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
	Slug string `json:"team_slug"`
}

type Dataset struct {
	Name string
	Slug string
	Team *Team
}

func (d *Dataset) UID() string {
	return fmt.Sprintf("%s-%s", d.Team.Slug, d.Slug)
}

func (d *Dataset) URL() string {
	path := path.Join(d.Team.Slug, "home", d.Slug)

	u, _ := web.Parse(path)

	return u.String()
}

func fetchDatasets(ctx context.Context, key string) ([]*Dataset, error) {
	t, err := fetchTeam(ctx, key)

	if err != nil {
		return nil, err
	}

	req, err := request(ctx, datasets, key)

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

func fetchTeam(ctx context.Context, key string) (*Team, error) {
	req, err := request(ctx, team, key)

	if err != nil {
		return nil, err
	}

	var t Team

	if err := response(ctx, req, &t); err != nil {
		return nil, err
	}

	return &t, nil
}

func request(ctx context.Context, path, key string) (*http.Request, error) {
	url, err := api.Parse(path)

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
