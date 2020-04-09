package amigo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aau-network-security/haaukins/store"
	pb "github.com/aau-network-security/haaukins/store/proto"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	"google.golang.org/grpc"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	address     = "localhost:50051"
)

func TestVerifyFlag(t *testing.T) {
	skey := "testing"

	dialer, close := store.CreateTestServer()
	defer close()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithDialer(dialer),
		grpc.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewStoreClient(conn)

	ts, err := store.NewEventStore(store.EventConfig{
		Name:           "Test Event",
		Tag:            "test",
		Available:      1,
		Capacity:       2,
		Lab:            store.Lab{},
		StartedAt:      nil,
		FinishExpected: nil,
		FinishedAt:     nil,
	}, client)

	var chals = []store.FlagConfig{
		{
			Tag:         "test",
			Name:        "Test Challenge",
			EnvVar:      "",
			Static:      "",
			Points:      10,
			Description: "",
			Category:    "",
		},
	}

	var flag store.Flag

	for _, team := range ts.GetTeams() {
		for _, chal := range chals {
			tag, _ := store.NewTag(string(chal.Tag))
			flag, _ = team.AddChallenge(store.Challenge{
				Tag:   tag,
				Name:  chal.Name,
				Value: store.NewFlag().String(),
			})
		}
	}

	team, err := ts.GetTeamByEmail("test@test.dk")
	if err != nil {
		t.Fatalf("unable to get the team by email: %v", err)
	}

	validToken, err := store.GetTokenForTeam([]byte(skey), team)
	if err != nil {
		t.Fatalf("unable to get token: %s", err)
	}
	validCookie := &http.Cookie{Name: "session", Value: validToken}

	tt := []struct {
		name   string
		input  string
		cookie *http.Cookie
		opts   []amigo.AmigoOpt
		err    string
	}{
		{
			name:   "too large",
			input:  `{"flag": "too-large"}`,
			cookie: validCookie,
			opts:   []amigo.AmigoOpt{amigo.WithMaxReadBytes(0)},
			err:    "request body is too large",
		},
		{
			name:  "unauthorized",
			input: `{"flag": "some-flag"}`,
			err:   "requires authentication",
		},
		{
			name:   "valid flag",
			cookie: validCookie,
			input:  fmt.Sprintf(`{"flag": "%s", "tag": "%s"}`, flag.String(), chals[0].Tag),
		},
		{
			name:   "unknown flag",
			cookie: validCookie,
			input:  `{"flag": "whatever-flag"}`,
			err:    "invalid flag",
		},
		{
			name:   "already taken flag",
			cookie: validCookie,
			input:  fmt.Sprintf(`{"flag": "%s", "tag": "%s"}`, flag.String(), chals[0].Tag),
			err:    "Flag is already completed",
		},
	}

	type reply struct {
		Err    string `json:"error,omitempty"`
		Status string `json:"status"`
	}


	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			am := amigo.NewAmigo(ts, chals, tc.opts...)
			srv := httptest.NewServer(am.Handler(nil, http.NewServeMux()))

			req, err := http.NewRequest("POST", srv.URL+"/flags/verify", bytes.NewBuffer([]byte(tc.input)))
			if err != nil {
				t.Fatalf("could not create request: %s", err)
			}
			req.Header.Add("Content-Type", "application/json")
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("could not perform request: %s", err)
			}
			defer resp.Body.Close()

			content, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("unable to read response body: %s", err)
			}

			var r reply
			if err := json.Unmarshal(content, &r); err != nil {
				t.Fatalf("unable to read json response (%s): %s", string(content), err)
			}

			if tc.err != "" {
				if r.Err != tc.err {
					t.Fatalf("unexpected error (%s), expected: %s", r.Err, tc.err)
				}
				return
			}

			if r.Err != "" {
				t.Fatalf("expected no errors to occur, but received: %s", r.Err)
			}

			if r.Status != "ok" {
				t.Fatalf("unexpected status: %s", r.Status)
			}
		})
	}
}
