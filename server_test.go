package fdbtest_test

import (
	"testing"

	"devzero.io/fdbtest"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/stretchr/testify/assert"
)

func init() {
	fdb.MustAPIVersion(620)
}

func BenchmarkRoundtrip(b *testing.B) {
	for i := 0; i < b.N; i++ {
		node := fdbtest.MustStart(b)
		node.Destroy(b)
	}
}

func TestRoundtrip(t *testing.T) {
	// start foundationdb node
	node := fdbtest.MustStart(t)

	// get the database
	db := node.DB

	// set foo key to bar
	_, err := db.Transact(func(tx fdb.Transaction) (interface{}, error) {
		tx.Set(fdb.Key("foo"), []byte("bar"))
		return nil, nil
	})
	assert.NoError(t, err)

	// get foo key
	value, err := db.Transact(func(tx fdb.Transaction) (interface{}, error) {
		return tx.Get(fdb.Key("foo")).Get()
	})

	// assert result
	assert.NoError(t, err)
	assert.Equal(t, []byte("bar"), value)
}
