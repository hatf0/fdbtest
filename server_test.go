package fdbtest_test

import (
	"testing"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/pjvds/fdbtest"
)

func TestRoundtrip(t *testing.T) {
	fdb.MustAPIVersion(610)

	// start foundationdb node
	node := fdbtest.MustStart()
	defer node.Destroy()

	// open fdb.Database
	db := node.MustOpenDB()

	// set foo key to bar
	_, err := db.Transact(func(tx fdb.Transaction) (interface{}, error) {
		tx.Set(fdb.Key("foo"), []byte("bar"))
		return nil, nil
	})
	if err != nil {
		t.Fatalf("set foo key failed: %v", err.Error())
	}

	// get foo key
	value, err := db.Transact(func(tx fdb.Transaction) (interface{}, error) {
		return tx.Get(fdb.Key("foo")).Get()
	})
	if err != nil {
		t.Fatalf("get foo key failed: %v", err.Error())
	}

	// assert foo value
	if "bar" != string(value.([]byte)) {
		t.Fatalf("expected bar, got %v", string(value.([]byte)))
	}
}
