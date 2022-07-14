package main

import (
	"errors"
	"os"
)

func PartitionBy[T any, K comparable](collection []T, iteratee func(x T) K) map[K][]T {
	res := make(map[K][]T)
	for _, item := range collection {
		key := iteratee(item)
		res[key] = append(res[key], item)
	}
	return res
}

func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}
