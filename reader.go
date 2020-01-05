package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	badger "github.com/dgraph-io/badger"
)

func main() {

	storagePath := flag.String("storage", "./data/docker-search-colly", "storage path")
	flag.Parse()

	store, err := badger.Open(badger.DefaultOptions(*storagePath))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	err = store.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			err := item.Value(func(v []byte) error {
				if strings.HasSuffix(string(k), "/dockerfile") {
					fmt.Printf("key=%s, value=%s\n", k, v)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

}
