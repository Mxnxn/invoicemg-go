package mongostore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

// colUserSettings is the collection Mongoose gives Model("UserSetting"): lower-cased and
// pluralised. Both services read and write the same documents while they run side by side.
const colUserSettings = "usersettings"

type settings struct{ db *mongo.Database }

func (s *Store) Settings() store.Settings { return &settings{db: s.db} }

// Get reads one preference blob. A missing document is (nil, nil) - the first-login case Node
// answers with data:null. A present document always carries both fields (the schema default is
// {}), so a section that has never been written reads as {} rather than null, matching
// setDefaultsOnInsert.
func (s *settings) Get(ctx context.Context, ownerID store.ID, section store.SettingsSection) (json.RawMessage, error) {
	oid, err := objectID(ownerID)
	if err != nil {
		return nil, err
	}
	var doc bson.M
	err = s.db.Collection(colUserSettings).
		FindOne(ctx, bson.M{"owner_id": oid}).
		Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading user settings: %w", err)
	}
	return sectionJSON(doc[string(section)])
}

// Set upserts one blob and returns the stored value. $setOnInsert seeds owner_id, timestamps,
// __v and the OTHER section as {} on the insert that creates the row - so an update to
// appearance leaves a later /tables read answering {} rather than null, exactly as Mongoose's
// setDefaultsOnInsert does. On an update of an existing row $setOnInsert is ignored and the
// other section is left untouched.
func (s *settings) Set(ctx context.Context, ownerID store.ID, section store.SettingsSection, value json.RawMessage) (json.RawMessage, error) {
	oid, err := objectID(ownerID)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(value, &v); err != nil {
		return nil, fmt.Errorf("decoding settings value: %w", err)
	}
	now := time.Now().UTC()
	other := store.SettingsTables
	if section == store.SettingsTables {
		other = store.SettingsAppearance
	}
	update := bson.M{
		"$set": bson.M{string(section): v, "updatedAt": now},
		"$setOnInsert": bson.M{
			"owner_id":    oid,
			"createdAt":   now,
			"__v":         0,
			string(other): bson.M{},
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var doc bson.M
	err = s.db.Collection(colUserSettings).
		FindOneAndUpdate(ctx, bson.M{"owner_id": oid}, update, opts).
		Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("saving user settings: %w", err)
	}
	return sectionJSON(doc[string(section)])
}

// sectionJSON marshals a decoded Mixed value to the JSON the wire carries. A section that is
// somehow absent or explicitly null is treated as the empty object the schema defaults to, so
// the response is {} and never a bare null for a row that exists.
func sectionJSON(v any) (json.RawMessage, error) {
	if v == nil {
		return json.RawMessage("{}"), nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encoding settings value: %w", err)
	}
	return b, nil
}
