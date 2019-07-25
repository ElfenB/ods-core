package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestMakePipelineName(t *testing.T) {
	tests := []struct {
		project   string
		component string
		branch    string
		expected  string
	}{
		{
			"PRJ",
			"comp",
			"bugfix/PRJ-529-bar-6-baz",
			"comp-529",
		},
		{ // also allow a bare ticket id.
			"PRJ",
			"comp",
			"bugfix/PRJ-529",
			"comp-529",
		},
		{
			// case insensitive comparison of project
			"pRJ",
			"comp",
			"bugfix/Prj-529-bar-6-baz",
			"comp-529",
		},
		{
			// case insensitive comparison of component
			// component appears in lowercase in pipeline
			"PRJ",
			"ComP",
			"bugfix/PRJ-529-bar-6-baz",
			"comp-529",
		},
		{
			// missing - project in request
			"",
			"comp",
			"bugfix/PRJ-529-bar-6-baz",
			"comp-6",
		},
		{
			"PRJ",
			"comp",
			"bugfix/äü",
			"comp-bugfix-",
		},
		{
			// assert current behavior, but could be changed in the future.
			"PRJ",
			"comp",
			"bugfix/PRJ-529-PRJ-777-bar-6-baz",
			"comp-777",
		},
		{
			// assert current behavior, but could be changed in the future.
			"PRJ",
			"comp",
			"bugfix/PRJ-529/PRJ-777-bar-6-baz",
			"comp-777",
		},
		// Cases which leads to pipeline string of
		// {comp}-{sanitized(branch)}
		{
			// project is a prefix or project in branch
			"PR",
			"comp",
			"bugfix/PRJ-529-bar-6-baz",
			"comp-bugfix-prj-529-bar-6-baz",
		},
		{
			// project in branch is a prefix of project
			"PRJL",
			"comp",
			"bugfix/PRJ-529-bar-6-baz",
			"comp-bugfix-prj-529-bar-6-baz",
		},
		{
			// missing '-' between project and number
			"PRJ",
			"comp",
			"bugfix/PRJ529-bar-6-baz",
			"comp-bugfix-prj529-bar-6-baz",
		},
	}

	for _, tt := range tests {
		pipeline := makePipelineName(tt.project, tt.component, tt.branch)
		if tt.expected != pipeline {
			t.Errorf(
				"Expected '%s' but '%s' returned by makePipeline(project='%s', component='%s', branch='%s')",
				tt.expected,
				pipeline,
				tt.project,
				tt.component,
				tt.branch,
			)
		}
	}
}

func TestIsProtectedBranch(t *testing.T) {
	tests := []struct {
		protectedBranchs []string
		branch           string
		expected         bool
	}{
		{
			[]string{"master"},
			"develop",
			false,
		},
		{
			[]string{"master", "develop"},
			"develop",
			true,
		},
		{
			[]string{"*"},
			"develop",
			true,
		},
		{
			[]string{"master", "release/"},
			"release/v1",
			true,
		},
		{
			[]string{"master", "release/"},
			"release",
			false,
		},
		{
			[]string{"hotfix/"},
			"feature/v2",
			false,
		},
	}

	for _, tt := range tests {
		actual := isProtectedBranch(tt.protectedBranchs, tt.branch)
		if tt.expected != actual {
			t.Errorf(
				"Expected '%v' but '%v' returned by isProtectedBranch(protectedBranchs='%s', branch='%s')",
				tt.expected,
				actual,
				tt.protectedBranchs,
				tt.branch,
			)
		}
	}
}

type mockClient struct {
	Event *Event
}

func (c *mockClient) Forward(e *Event) error {
	c.Event = e
	return nil
}
func (c *mockClient) CreatePipelineIfRequired(e *Event) error {
	c.Event = e
	return nil
}
func (c *mockClient) DeletePipeline(e *Event) error {
	c.Event = e
	return nil
}

func testServer() (*httptest.Server, *mockClient) {
	mc := &mockClient{}
	server = &Server{
		Client:            mc,
		Project:           "bar",
		Namespace:         "bar-cd",
		TriggerSecret:     "s3cr3t",
		ProtectedBranches: []string{"baz"},
		RepoBase:          "https://domain.com",
	}
	return httptest.NewServer(server.HandleRoot()), mc
}

func TestHandleRootRequiresTriggerSecret(t *testing.T) {
	ts, _ := testServer()
	defer ts.Close()

	f, err := os.Open("test-fixtures/repo-refs-changed-payload.json")
	if err != nil {
		t.Error(err)
		return
	}
	res, err := http.Post(ts.URL, "application/json", f)
	if err != nil {
		t.Error(err)
		return
	}
	expected := http.StatusUnauthorized
	actual := res.StatusCode
	if expected != actual {
		t.Errorf("Got status %v, want %v", actual, expected)
	}
}

func TestHandleRootReadsRequests(t *testing.T) {
	ts, mc := testServer()
	defer ts.Close()

	// The expected events depend on the values in the payload files.
	examples := []struct {
		payloadFile   string
		expectedEvent *Event
	}{
		{
			"repo-refs-changed-payload.json",
			&Event{
				Kind:      "forward",
				Project:   "bar",
				Namespace: "bar-cd",
				Repo:      "repository",
				Component: "repository",
				Branch:    "master",
				Pipeline:  "repository-master",
			},
		},
		{
			"pr-merged-payload.json",
			&Event{
				Kind:      "delete",
				Project:   "bar",
				Namespace: "bar-cd",
				Repo:      "repository",
				Component: "repository",
				Branch:    "admin/file-1505781548644",
				Pipeline:  "repository-admin-file-1505781548644",
			},
		},
		{
			"pr-declined-payload.json",
			&Event{
				Kind:      "delete",
				Project:   "bar",
				Namespace: "bar-cd",
				Repo:      "repository",
				Component: "repository",
				Branch:    "decline-me",
				Pipeline:  "repository-decline-me",
			},
		},
	}

	for _, example := range examples {
		f, err := os.Open("test-fixtures/" + example.payloadFile)
		if err != nil {
			t.Error(err)
			return
		}
		// Use secret defined in fake server.
		res, err := http.Post(ts.URL+"?trigger_secret=s3cr3t", "application/json", f)
		if err != nil {
			t.Error(err)
			return
		}
		_, err = ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Error(err)
		}

		expected := http.StatusOK
		actual := res.StatusCode
		if expected != actual {
			t.Errorf("Got status: %v, want: %v", actual, expected)
		}

		// RequestID cannot be known in advance, so set it now from actual value.
		example.expectedEvent.RequestID = mc.Event.RequestID
		if !reflect.DeepEqual(example.expectedEvent, mc.Event) {
			t.Errorf("Got event: %v, want: %v", mc.Event, example.expectedEvent)
		}
	}
}
