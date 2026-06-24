//go:build !nomongo

package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestGetDeterministicObjectID(t *testing.T) {
	browser := "Chrome"
	profile := "Default"
	timestamp := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	urlStr := "https://google.com"

	id1 := getDeterministicObjectID(browser, profile, timestamp, urlStr)
	id2 := getDeterministicObjectID(browser, profile, timestamp, urlStr)

	if id1 != id2 {
		t.Errorf("expected deterministic ObjectIDs, but got %v and %v", id1, id2)
	}

	id3 := getDeterministicObjectID(browser, profile, timestamp, "https://google.com/other")
	if id1 == id3 {
		t.Errorf("expected different ObjectIDs for different URLs, but they were identical")
	}

	if len(id1) != 12 {
		t.Errorf("expected 12 bytes ObjectID, got %d bytes", len(id1))
	}
}

func TestEToDocList(t *testing.T) {
	entries := []models.HistoryEntry{
		{
			Browser:       "Chrome",
			Profile:       "Default",
			Timestamp:     time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			URL:           "https://google.com",
			VisitDuration: 100,
		},
		{
			Browser:   "Firefox",
			Profile:   "Profile1",
			Timestamp: time.Date(2026, 6, 20, 12, 5, 0, 0, time.UTC),
			URL:       "https://firefox.com",
			VisitType: 3,
		},
		{
			Browser:        "Safari",
			Profile:        "Default",
			Timestamp:      time.Date(2026, 6, 20, 12, 10, 0, 0, time.UTC),
			URL:            "https://apple.com",
			LoadSuccessful: func() *bool { b := true; return &b }(),
		},
	}

	jobs := eToDocList(entries, "merged", false)
	if len(jobs) != 3 || len(jobs[0].colls) != 1 || jobs[0].colls[0] != "history" {
		t.Errorf("unexpected merged jobs: %+v", jobs)
	}
	if _, hasDuration := jobs[0].doc["visit_duration"]; hasDuration {
		t.Errorf("expected relational merged job to exclude visit_duration")
	}

	jobs = eToDocList(entries, "merged", true)
	if _, hasDuration := jobs[0].doc["visit_duration"]; !hasDuration {
		t.Errorf("expected flat merged job to include visit_duration")
	}

	jobs = eToDocList(entries, "split", false)
	if len(jobs) != 3 || jobs[0].colls[0] != "history_chrome" {
		t.Errorf("unexpected split jobs: %+v", jobs)
	}

	jobs = eToDocList(entries, "both", true)
	if len(jobs) != 6 {
		t.Errorf("expected 6 jobs in both flat mode, got %d", len(jobs))
	}

	jobs = eToDocList(entries, "both", false)
	if len(jobs) != 6 {
		t.Errorf("expected 6 jobs in both relational mode, got %d", len(jobs))
	}
}

type mockMongoClient struct {
	db *mockMongoDatabase
}

func (c *mockMongoClient) Database(name string) mongoDatabase {
	return c.db
}

func (c *mockMongoClient) Disconnect(ctx context.Context) error {
	return nil
}

type mockMongoDatabase struct {
	collections map[string]*mockMongoCollection
}

func (d *mockMongoDatabase) Collection(name string) mongoCollection {
	if _, ok := d.collections[name]; !ok {
		d.collections[name] = &mockMongoCollection{
			models: []mongo.WriteModel{},
		}
	}
	return d.collections[name]
}

type mockMongoCollection struct {
	models []mongo.WriteModel
}

func (c *mockMongoCollection) Indexes() mongoIndexView {
	return &mockMongoIndexView{}
}

func (c *mockMongoCollection) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	c.models = append(c.models, models...)
	return &mongo.BulkWriteResult{
		UpsertedCount: int64(len(models)),
	}, nil
}

type mockMongoIndexView struct{}

func (v *mockMongoIndexView) CreateOne(ctx context.Context, model mongo.IndexModel, opts ...*options.CreateIndexesOptions) (string, error) {
	return "", nil
}

func TestIngestMongoDB_Success(t *testing.T) {
	oldNewMongoClient := newMongoClient
	mockColls := make(map[string]*mockMongoCollection)
	newMongoClient = func(ctx context.Context, uri string) (mongoClient, error) {
		return &mockMongoClient{
			db: &mockMongoDatabase{collections: mockColls},
		}, nil
	}
	defer func() { newMongoClient = oldNewMongoClient }()

	entries := []models.HistoryEntry{
		{
			Browser:   "Chrome",
			Profile:   "Default",
			Timestamp: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			URL:       "https://google.com",
		},
		{
			Browser:   "Firefox",
			Profile:   "Profile1",
			Timestamp: time.Date(2026, 6, 20, 12, 5, 0, 0, time.UTC),
			URL:       "https://firefox.com",
		},
		{
			Browser:   "Safari",
			Profile:   "Personal",
			Timestamp: time.Date(2026, 6, 20, 12, 10, 0, 0, time.UTC),
			URL:       "https://apple.com",
		},
	}

	count, err := Ingest("mongodb://localhost/testdb", entries, "skip", "both", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}

	count, err = Ingest("mongodb://localhost/testdb?connectTimeoutMS=1000", entries, "replace", "both", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}

	count, err = Ingest("mongodb://localhost/", entries, "skip", "split", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}
}

func TestIngestMongoDB_ConnectError(t *testing.T) {
	oldNewMongoClient := newMongoClient
	newMongoClient = func(ctx context.Context, uri string) (mongoClient, error) {
		return nil, errors.New("mock connection error")
	}
	defer func() { newMongoClient = oldNewMongoClient }()

	_, err := Ingest("mongodb://localhost/testdb", nil, "skip", "both", false)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestRealMongoWrappers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newMongoClient(context.Background(), "mongodb://%4")
	if err == nil {
		t.Errorf("expected Connect error for invalid URI, got nil")
	}

	client, err := newMongoClient(context.Background(), "mongodb://localhost:27017")
	if err != nil {
		t.Logf("mongo.Connect failed: %v", err)
		return
	}

	db := client.Database("testdb")
	coll := db.Collection("testcoll")
	indexes := coll.Indexes()

	_, _ = indexes.CreateOne(ctx, mongo.IndexModel{})
	_, _ = coll.BulkWrite(ctx, nil)
	_ = client.Disconnect(ctx)
}
