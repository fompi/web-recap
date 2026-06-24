//go:build !nomongo

package database

import (
	"context"
	"crypto/sha256"
	"net/url"
	"strings"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ingestMongoDB(ctx context.Context, uri string, entries []models.HistoryEntry, conflictStrategy, mode string, flat bool) (int, error) {
	client, err := newMongoClient(ctx, uri)
	if err != nil {
		return 0, err
	}
	defer client.Disconnect(ctx)

	dbName := "web_recap"
	u, err := url.Parse(uri)
	if err == nil && u.Path != "" {
		path := strings.TrimPrefix(u.Path, "/")
		if path != "" {
			dbName = path
		}
	}

	db := client.Database(dbName)
	insertedCount := 0

	// Gather unique index requirements for targeted collections
	ensureIndex := func(collName string) {
		coll := db.Collection(collName)
		// For relational child collections, they don't have browser, profile, etc.
		// Instead, they are unique on "_id" which MongoDB handles automatically.
		if collName == "history" || (mode == "split") || (mode == "both" && flat) {
			indexModel := mongo.IndexModel{
				Keys: bson.D{
					{Key: "browser", Value: 1},
					{Key: "profile", Value: 1},
					{Key: "timestamp", Value: 1},
					{Key: "url", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			}
			_, _ = coll.Indexes().CreateOne(ctx, indexModel)
		}
	}

	// Setup collections
	if mode == "merged" || mode == "both" {
		ensureIndex("history")
	}
	if mode == "split" || mode == "both" {
		for _, entry := range entries {
			ensureIndex(getBrowserSpecificTableName(entry.Browser))
		}
	}

	// Bulk write maps for each targeted collection
	bulks := make(map[string][]mongo.WriteModel)

	for _, entry := range eToDocList(entries, mode, flat) {
		colls := entry.colls
		doc := entry.doc

		var filter bson.D
		if _, ok := doc["browser"]; ok {
			filter = bson.D{
				{Key: "browser", Value: doc["browser"]},
				{Key: "profile", Value: doc["profile"]},
				{Key: "timestamp", Value: doc["timestamp"]},
				{Key: "url", Value: doc["url"]},
			}
		} else {
			filter = bson.D{
				{Key: "_id", Value: doc["_id"]},
			}
		}

		var model mongo.WriteModel
		switch conflictStrategy {
		case "skip":
			model = mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(bson.D{{Key: "$setOnInsert", Value: doc}}).SetUpsert(true)
		case "replace":
			model = mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(bson.D{{Key: "$set", Value: doc}}).SetUpsert(true)
		}

		for _, c := range colls {
			bulks[c] = append(bulks[c], model)
		}
	}

	// Execute bulks
	for collName, modelsList := range bulks {
		if len(modelsList) > 0 {
			coll := db.Collection(collName)
			res, err := coll.BulkWrite(ctx, modelsList, options.BulkWrite().SetOrdered(false))
			if err == nil && res != nil {
				insertedCount += int(res.UpsertedCount) + int(res.ModifiedCount)
			}
		}
	}

	if insertedCount > len(entries) {
		return len(entries), nil
	}
	return insertedCount, nil
}

type mongoDocJob struct {
	colls []string
	doc   bson.M
}

func eToDocList(entries []models.HistoryEntry, mode string, flat bool) []mongoDocJob {
	var jobs []mongoDocJob

	for _, entry := range entries {
		// Common document map
		commonDoc := bson.M{
			"browser":      entry.Browser,
			"profile":      entry.Profile,
			"timestamp":    entry.Timestamp,
			"url":          entry.URL,
			"title":        entry.Title,
			"domain":       entry.Domain,
			"visit_count":  entry.VisitCount,
			"scheme":       entry.Scheme,
			"username":     entry.Username,
			"fqdn":         entry.FQDN,
			"domain_name":  entry.DomainName,
			"subdomain":    entry.Subdomain,
			"tld":          entry.TLD,
			"port":         entry.Port,
			"path":         entry.Path,
			"query_params": entry.QueryParams,
		}

		// Extended document map (for flat or split collections)
		extDoc := bson.M{
			"browser":      entry.Browser,
			"profile":      entry.Profile,
			"timestamp":    entry.Timestamp,
			"url":          entry.URL,
			"title":        entry.Title,
			"domain":       entry.Domain,
			"visit_count":  entry.VisitCount,
			"scheme":       entry.Scheme,
			"username":     entry.Username,
			"fqdn":         entry.FQDN,
			"domain_name":  entry.DomainName,
			"subdomain":    entry.Subdomain,
			"tld":          entry.TLD,
			"port":         entry.Port,
			"path":         entry.Path,
			"query_params": entry.QueryParams,
			// Chrome
			"visit_duration": entry.VisitDuration,
			"transition":     entry.Transition,
			"from_visit":     entry.FromVisit,
			"segment_id":     entry.SegmentID,
			"typed_count":    entry.TypedCount,
			// Firefox
			"visit_type": entry.VisitType,
			"session":    entry.Session,
			"frequency":  entry.Frequency,
			"typed":      entry.Typed,
			// Safari
			"redirect_source":      entry.RedirectSource,
			"redirect_destination": entry.RedirectDestination,
			"origin":               entry.Origin,
			"generation_type":      entry.GenerationType,
			"load_successful":      entry.LoadSuccessful,
			"http_non_get":         entry.HTTPNonGET,
			"synthesized":          entry.Synthesized,
		}

		if mode == "merged" {
			if flat {
				jobs = append(jobs, mongoDocJob{colls: []string{"history"}, doc: extDoc})
			} else {
				jobs = append(jobs, mongoDocJob{colls: []string{"history"}, doc: commonDoc})
			}
		} else if mode == "split" {
			jobs = append(jobs, mongoDocJob{colls: []string{getBrowserSpecificTableName(entry.Browser)}, doc: extDoc})
		} else if mode == "both" {
			if flat {
				jobs = append(jobs, mongoDocJob{colls: []string{"history"}, doc: extDoc})
				jobs = append(jobs, mongoDocJob{colls: []string{getBrowserSpecificTableName(entry.Browser)}, doc: extDoc})
			} else {
				// Relational mode using deterministic ObjectIDs
				parentID := getDeterministicObjectID(entry.Browser, entry.Profile, entry.Timestamp, entry.URL)
				
				parentDoc := bson.M{
					"_id":          parentID,
					"browser":      entry.Browser,
					"profile":      entry.Profile,
					"timestamp":    entry.Timestamp,
					"url":          entry.URL,
					"title":        entry.Title,
					"domain":       entry.Domain,
					"visit_count":  entry.VisitCount,
					"scheme":       entry.Scheme,
					"username":     entry.Username,
					"fqdn":         entry.FQDN,
					"domain_name":  entry.DomainName,
					"subdomain":    entry.Subdomain,
					"tld":          entry.TLD,
					"port":         entry.Port,
					"path":         entry.Path,
					"query_params": entry.QueryParams,
				}

				var childDoc bson.M
				switch getBrowserClass(entry.Browser) {
				case "firefox":
					childDoc = bson.M{
						"_id":        parentID,
						"from_visit": entry.FromVisit,
						"visit_type": entry.VisitType,
						"session":    entry.Session,
						"frequency":  entry.Frequency,
						"typed":      entry.Typed,
					}
				case "safari":
					childDoc = bson.M{
						"_id":                  parentID,
						"redirect_source":      entry.RedirectSource,
						"redirect_destination": entry.RedirectDestination,
						"origin":              entry.Origin,
						"generation_type":      entry.GenerationType,
						"load_successful":      entry.LoadSuccessful,
						"http_non_get":          entry.HTTPNonGET,
						"synthesized":          entry.Synthesized,
					}
				default: // chrome and other chromium-based browsers
					childDoc = bson.M{
						"_id":            parentID,
						"visit_duration": entry.VisitDuration,
						"transition":     entry.Transition,
						"from_visit":     entry.FromVisit,
						"segment_id":     entry.SegmentID,
						"typed_count":    entry.TypedCount,
					}
				}

				jobs = append(jobs, mongoDocJob{colls: []string{"history"}, doc: parentDoc})
				jobs = append(jobs, mongoDocJob{colls: []string{getBrowserSpecificTableName(entry.Browser)}, doc: childDoc})
			}
		}
	}

	return jobs
}

func getDeterministicObjectID(browser, profile string, timestamp time.Time, urlStr string) primitive.ObjectID {
	h := sha256.New()
	h.Write([]byte(browser))
	h.Write([]byte{0})
	h.Write([]byte(profile))
	h.Write([]byte{0})
	h.Write([]byte(timestamp.UTC().Format(time.RFC3339Nano)))
	h.Write([]byte{0})
	h.Write([]byte(urlStr))
	var id [12]byte
	copy(id[:], h.Sum(nil)[:12])
	return id
}

type mongoClient interface {
	Database(name string) mongoDatabase
	Disconnect(ctx context.Context) error
}

type mongoDatabase interface {
	Collection(name string) mongoCollection
}

type mongoCollection interface {
	Indexes() mongoIndexView
	BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error)
}

type mongoIndexView interface {
	CreateOne(ctx context.Context, model mongo.IndexModel, opts ...*options.CreateIndexesOptions) (string, error)
}

var newMongoClient = func(ctx context.Context, uri string) (mongoClient, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	return &realMongoClient{client: client}, nil
}

type realMongoClient struct {
	client *mongo.Client
}

func (c *realMongoClient) Database(name string) mongoDatabase {
	return &realMongoDatabase{db: c.client.Database(name)}
}

func (c *realMongoClient) Disconnect(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

type realMongoDatabase struct {
	db *mongo.Database
}

func (d *realMongoDatabase) Collection(name string) mongoCollection {
	return &realMongoCollection{coll: d.db.Collection(name)}
}

type realMongoCollection struct {
	coll *mongo.Collection
}

func (c *realMongoCollection) Indexes() mongoIndexView {
	return &realMongoIndexView{view: c.coll.Indexes()}
}

func (c *realMongoCollection) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	return c.coll.BulkWrite(ctx, models, opts...)
}

type realMongoIndexView struct {
	view mongo.IndexView
}

func (v *realMongoIndexView) CreateOne(ctx context.Context, model mongo.IndexModel, opts ...*options.CreateIndexesOptions) (string, error) {
	return v.view.CreateOne(ctx, model, opts...)
}
