package common_test

import (
	"bytes"
	"fmt"
	common "github.com/ethereum/go-ethereum/swarmdb"
	"testing"
)

func TestENSSimulation(t *testing.T) {
	store, err := common.NewENSSimulation("/tmp/ens.db")
	if err != nil {
		t.Fatal("failure to open ENSSimulation")
	}
	indexName := []byte("contact")
	roothash := []byte("contactroothash")
	store.StoreRootHash(indexName, roothash)

	val, err := store.GetRootHash(indexName)
	if err != nil {
	}
	if bytes.Compare(val, roothash) != 0 {
		t.Fatal("Err1", indexName, roothash, val)
	} else {
		fmt.Printf("SUCCESS1:  %v => %v\n", string(indexName), string(val))
	}
}
